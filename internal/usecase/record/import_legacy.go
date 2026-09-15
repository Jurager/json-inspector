package record

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"json-inspector/internal/domain"
)

// LegacySource is the key the old frontend kept its history under.
const LegacySource = "localStorage:ji-history-v1"

// ImportReport says what the import did, so the log and the UI can mention it once.
type ImportReport struct {
	Records   int      `json:"records"`
	Skipped   int      `json:"skipped"`
	Warnings  []string `json:"warnings,omitempty"`
	Completed bool     `json:"completed"`
}

// legacyRecord is the shape the TypeScript store persisted: its field names, spelled the same way.
// Only what the app draws is read — anything else in the old rows is dropped rather than guessed
// at.
type legacyRecord struct {
	ID              string             `json:"id"`
	Method          string             `json:"method"`
	URL             string             `json:"url"`
	Status          int                `json:"status"`
	StatusText      string             `json:"statusText"`
	ContentType     string             `json:"contentType"`
	Error           string             `json:"error"`
	DurationMs      int64              `json:"durationMs"`
	StartedAt       int64              `json:"startedAt"`
	Source          string             `json:"source"`
	DNSMs           int64              `json:"dnsMs"`
	ConnectMs       int64              `json:"connectMs"`
	TLSMs           int64              `json:"tlsMs"`
	WaitMs          int64              `json:"waitMs"`
	DownloadMs      int64              `json:"downloadMs"`
	HasTiming       bool               `json:"hasTiming"`
	TabID           int                `json:"tabId"`
	TabTitle        string             `json:"tabTitle"`
	TabURL          string             `json:"tabURL"`
	FavIconURL      string             `json:"favIconUrl"`
	RequestHeaders  legacyHeaders      `json:"requestHeaders"`
	ResponseHeaders legacyHeaders      `json:"responseHeaders"`
	RequestBody     string             `json:"requestBody"`
	ResponseBody    string             `json:"responseBody"`
	RequestCookies  []domain.CookieRow `json:"requestCookies"`
}

// legacyHeaders reads both shapes this field ever had: an object of names to values, which is what
// the oldest rows hold, and the array of pairs the record has used since headers started repeating.
type legacyHeaders []domain.HeaderPair

func (h *legacyHeaders) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	if data[0] == '{' {
		values := map[string]string{}
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
		// An object has no order of its own, so sorting is what keeps two runs of the import
		// identical — the old build's own order is not recoverable from a map anyway.
		names := make([]string, 0, len(values))
		for name := range values {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			*h = append(*h, domain.HeaderPair{Name: name, Value: values[name]})
		}
		return nil
	}

	var pairs []domain.HeaderPair
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}
	*h = pairs
	return nil
}

// ImportLegacy moves the history the old frontend kept in localStorage into the database. It runs
// once: the claim in data_imports is what makes it once, and a run that failed is retried on the
// next launch because its rows are the only copy of that history.
func (u *UseCase) ImportLegacy(ctx context.Context, raw string) (ImportReport, error) {
	var report ImportReport

	// What the old frontend kept in localStorage lands in the default workspace: this is a one-time
	// repair of what the installation had before history moved into the database, and it must not
	// follow the user around spaces.
	const workspace = domain.WorkspacePersonalID

	claimed, err := u.store.ClaimImport(ctx, LegacySource)
	if err != nil {
		return report, err
	}
	if !claimed {
		return report, nil
	}

	records, skipped, err := parseLegacy(raw)
	if err != nil {
		finish := u.store.FinishImport(ctx, LegacySource, "failed", err.Error())
		if finish != nil {
			log.Printf("[import] recording the failure: %v", finish)
		}
		return report, fmt.Errorf("reading ji-history-v1: %w", err)
	}
	report.Skipped = skipped

	if len(records) == 0 {
		if err := u.store.FinishImport(ctx, LegacySource, "done", "nothing to import"); err != nil {
			return report, err
		}
		return report, nil
	}

	for _, rec := range records {
		if err := u.store.SaveRecord(ctx, workspace, rec); err != nil {
			// One bad row must not cost the rest of the history: it is a warning, and the import
			// carries on to the end.
			report.Skipped++
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("record %s was not imported: %v", rec.ID, err))
			continue
		}
		report.Records++
	}

	detail, err := json.Marshal(report)
	if err != nil {
		detail = []byte("{}")
	}
	if err := u.store.FinishImport(ctx, LegacySource, "done", string(detail)); err != nil {
		return report, err
	}
	report.Completed = true
	return report, nil
}

// parseLegacy reads the old array. A row without a method or a URL is not a request and is skipped;
// everything else is carried over as it was stored.
func parseLegacy(raw string) ([]domain.Record, int, error) {
	if raw == "" {
		return nil, 0, nil
	}

	var legacy []legacyRecord
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return nil, 0, err
	}

	out := make([]domain.Record, 0, len(legacy))
	skipped := 0
	for _, old := range legacy {
		if old.ID == "" || old.URL == "" {
			skipped++
			continue
		}

		source := domain.SourceManual
		if old.Source == string(domain.SourceBrowser) {
			source = domain.SourceBrowser
		}

		out = append(out, domain.Record{
			RecordSummary: domain.RecordSummary{
				ID:          old.ID,
				Source:      source,
				Method:      old.Method,
				URL:         old.URL,
				Status:      old.Status,
				StatusText:  old.StatusText,
				ContentType: old.ContentType,
				Error:       old.Error,
				DurationUs:  old.DurationMs * 1000,
				StartedAt:   old.StartedAt,
				TabID:       old.TabID,
				TabTitle:    old.TabTitle,
				TabURL:      old.TabURL,
				FavIconURL:  old.FavIconURL,
			},
			DNSUs:           millisToMicros(old.DNSMs),
			ConnectUs:       millisToMicros(old.ConnectMs),
			TLSUs:           millisToMicros(old.TLSMs),
			WaitUs:          millisToMicros(old.WaitMs),
			DownloadUs:      millisToMicros(old.DownloadMs),
			RequestHeaders:  orEmptyPairs(old.RequestHeaders),
			ResponseHeaders: orEmptyPairs(old.ResponseHeaders),
			RequestCookies:  old.RequestCookies,
			// The legacy store kept what it had and never said whether it was short; nothing here can
			// claim otherwise.
			RequestBody:   bodyRef(old.RequestBody, false),
			ResponseBody:  bodyRef(old.ResponseBody, false),
			RequestBytes:  int64(len(old.RequestBody)),
			ResponseBytes: int64(len(old.ResponseBody)),
		})
	}
	return out, skipped, nil
}
