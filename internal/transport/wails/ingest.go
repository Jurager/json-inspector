package wails

import (
	"context"
	"log"
	"strings"

	"json-inspector/internal/domain"
	"json-inspector/internal/transport/bridge"
	"json-inspector/internal/usecase/record"
)

// Deliberately not a bound service: Wails binds every exported method of a service, and these are
// callbacks rather than an API the frontend may call.
type captureIngest struct {
	host    *Host
	records *record.UseCase
}

func newCaptureIngest(host *Host, records *record.UseCase) bridge.Ingest {
	return captureIngest{host: host, records: records}
}

// The window hears about this as the same event, and the same record, as one this app sent.
func (c captureIngest) Captured(req bridge.CapturedRequest) {
	responseHeaders := domain.PairsFromMap(req.ResponseHeaders)

	if _, err := c.records.Ingest(context.Background(), record.IngestInput{
		Source:          domain.SourceBrowser,
		Method:          req.Method,
		URL:             req.URL,
		Status:          req.Status,
		StatusText:      req.StatusText,
		ContentType:     headerValue(responseHeaders, "Content-Type"),
		RequestHeaders:  domain.PairsFromMap(req.RequestHeaders),
		ResponseHeaders: responseHeaders,
		RequestBody:     req.RequestBody,
		ResponseBody:    req.ResponseBody,
		DurationMs:      req.DurationMs,
		StartedAt:       req.StartedAt,
		WaitMs:          req.WaitMs,
		DownloadMs:      req.DownloadMs,
		TabID:           req.TabID,
		TabTitle:        req.TabTitle,
		TabURL:          req.TabURL,
		FavIconURL:      req.FavIconURL,
	}); err != nil {
		// A capture that cannot be stored is a gap in the list, not a reason to drop the socket:
		// the extension has no way to retry it, and the next request is already on its way.
		log.Printf("[capture] storing %s %s: %v", req.Method, req.URL, err)
	}
}

func (c captureIngest) StateChanged(state bridge.CaptureState) {
	c.host.Emit(eventCaptureState, state)
}

func (c captureIngest) ClientGone() {
	c.host.Emit(eventCaptureDisconnected)
}

func (c captureIngest) FocusRequested(req bridge.FocusRequest) {
	c.host.FocusMain()
	if req.Tab > 0 {
		c.host.OpenTab(req.Tab)
	}
}

// headerValue reads one name out of headers, ignoring case: the extension reports the names the
// browser used, which are lower-case.
func headerValue(headers []domain.HeaderPair, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}
