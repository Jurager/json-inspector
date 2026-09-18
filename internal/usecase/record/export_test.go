package record

import (
	"context"
	"encoding/json"
	"testing"

	"json-inspector/internal/domain"
)

// A saved session holds the traffic of one tab and nobody else's: the file is what somebody
// attaches to a report about a page, and requests from the tabs beside it are a different story.
func TestExportHARTakesOneTab(t *testing.T) {
	uc, store, _, _ := newUseCase()
	ctx := context.Background()

	store.saved = []domain.Record{
		browserRecord("rec-2", 7, "https://api.example.com/second"),
		browserRecord("rec-1", 7, "https://api.example.com/first"),
		browserRecord("other", 8, "https://api.example.com/elsewhere"),
	}
	store.bodies["rec-1"] = `{"data":[]}`
	store.bodies["rec-2"] = `{"data":[1]}`

	data, name, err := uc.ExportHAR(ctx, "7")
	if err != nil {
		t.Fatalf("ExportHAR: %v", err)
	}
	if name != "api.example.com" {
		t.Errorf("name = %q, want the page's host", name)
	}

	var doc struct {
		Log struct {
			Entries []struct {
				Request struct{ URL string } `json:"request"`
			} `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if len(doc.Log.Entries) != 2 {
		t.Fatalf("entries = %d, want the two of that tab", len(doc.Log.Entries))
	}
	// Oldest first: a session is a file about the order things happened in, and the list it is built
	// from is newest-first because that is what a panel shows.
	if doc.Log.Entries[0].Request.URL != "https://api.example.com/first" {
		t.Errorf("first entry = %q, want the earliest request", doc.Log.Entries[0].Request.URL)
	}

	// The list the panel draws is capped; this read is not, and a file that quietly stopped at two
	// hundred requests would be a file that lies about the session.
	if len(store.listed) == 0 || store.listed[len(store.listed)-1] != 0 {
		t.Errorf("asked for %v, want the export to read without a limit", store.listed)
	}
}

// A tab with no traffic is a file with no entries rather than a refusal: the button was pressed,
// and an empty session is an answer about what was captured.
func TestExportHAROfATabWithNothing(t *testing.T) {
	uc, store, _, _ := newUseCase()
	store.saved = []domain.Record{browserRecord("rec-1", 7, "https://api.example.com/first")}

	data, _, err := uc.ExportHAR(context.Background(), "8")
	if err != nil {
		t.Fatalf("ExportHAR: %v", err)
	}

	var doc harDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if len(doc.Log.Entries) != 0 {
		t.Errorf("entries = %d, want none for a tab that captured nothing", len(doc.Log.Entries))
	}
}

// The smallest shape of the document this test reads: what it asserts is the count, and the rest of
// the file is the writer's business.
type harDocument struct {
	Log struct {
		Entries []json.RawMessage `json:"entries"`
	} `json:"log"`
}

func browserRecord(id string, tabID int, url string) domain.Record {
	return domain.Record{
		RecordSummary: domain.RecordSummary{
			ID: id, Source: domain.SourceBrowser, Method: "GET", URL: url, Status: 200,
			StartedAt: 1_700_000_000_000, TabID: tabID, TabTitle: "Blog demo",
			TabURL: "https://api.example.com/articles",
		},
	}
}
