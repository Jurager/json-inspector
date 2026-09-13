package sqlite

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"json-inspector/internal/domain"
)

func sampleRecord(id string, source domain.RecordSource) domain.Record {
	return domain.Record{
		RecordSummary: domain.RecordSummary{
			ID:         id,
			Source:     source,
			Method:     "GET",
			URL:        "https://api.example.com/articles",
			Status:     200,
			StatusText: "200 OK",
			DurationMs: 42,
			StartedAt:  time.Now().UnixMilli(),
			HasTiming:  true,
			TabID:      7,
			TabTitle:   "Example",
		},
		DNSMs:      1,
		ConnectMs:  2,
		TLSMs:      3,
		WaitMs:     4,
		DownloadMs: 5,
		RequestHeaders: []domain.HeaderPair{
			{Name: "Accept", Value: "application/vnd.api+json"},
		},
		ResponseHeaders: []domain.HeaderPair{
			{Name: "Content-Type", Value: "application/vnd.api+json"},
			{Name: "Set-Cookie", Value: "a=1"},
			{Name: "Set-Cookie", Value: "b=2"},
		},
		RequestCookies: []domain.CookieRow{{Name: "session", Value: "abc", Path: "/"}},
		RequestBody:    &domain.BodyRef{Inline: `{"a":1}`, Size: 7},
		ResponseBody:   &domain.BodyRef{Inline: `{"data":[]}`, Size: 11},
	}
}

func TestRecordRoundTrip(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	saved := sampleRecord("rec-1", domain.SourceManual)
	saved.ResponseBytes = 11
	if err := store.SaveRecord(ctx, saved); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}

	list, err := store.Records(ctx, domain.SourceManual, 0)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("records = %d, want one", len(list))
	}
	row := list[0]
	if row.Method != "GET" || row.Status != 200 || row.TabTitle != "Example" || !row.HasTiming {
		t.Errorf("record = %+v, want the saved fields", row)
	}
	if row.WaitMs != 4 || row.ResponseBytes != 11 {
		t.Errorf("phases = %+v, want them back with the list", row)
	}
	// Repeated names survive, in the order they came in: two Set-Cookie lines are two cookies.
	var cookies []string
	for _, h := range row.ResponseHeaders {
		if h.Name == "Set-Cookie" {
			cookies = append(cookies, h.Value)
		}
	}
	if len(cookies) != 2 || cookies[0] != "a=1" || cookies[1] != "b=2" {
		t.Errorf("Set-Cookie = %v, want both in order", cookies)
	}
	if len(row.RequestCookies) != 1 || row.RequestCookies[0].Name != "session" {
		t.Errorf("cookies = %+v, want the saved jar row", row.RequestCookies)
	}

	// The list names each body and stops there: this is what keeps two hundred records from being
	// two hundred documents.
	if row.ResponseBody == nil || row.ResponseBody.Size != 11 || row.ResponseBody.Inline != "" {
		t.Errorf("responseBody = %+v, want its size and no text", row.ResponseBody)
	}
	if row.RequestBody == nil || row.RequestBody.Size != 7 {
		t.Errorf("requestBody = %+v", row.RequestBody)
	}
	for _, side := range []struct {
		kind domain.BodySide
		want string
	}{
		{domain.SideRequest, `{"a":1}`},
		{domain.SideResponse, `{"data":[]}`},
	} {
		text, err := store.ReadBody(ctx, "rec-1", side.kind)
		if err != nil || text != side.want {
			t.Errorf("%s body = %q, %v; want %q", side.kind, text, err, side.want)
		}
	}
}

// A body is stored whole and read back whole, however large it is: the size in the list is a label,
// not a limit.
func TestBodyIsReadBackWhole(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	big := strings.Repeat("x", domain.InlineBodyLimit+1)
	rec := sampleRecord("rec-big", domain.SourceManual)
	rec.ResponseBody = &domain.BodyRef{Inline: big, Size: int64(len(big))}
	// Nothing was sent, so the request side is unreachable — which the last check uses.
	rec.RequestBody = nil
	if err := store.SaveRecord(ctx, rec); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}

	list, err := store.Records(ctx, "", 0)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if list[0].ResponseBody == nil || list[0].ResponseBody.Size != int64(len(big)) {
		t.Errorf("responseBody = %+v, want its size", list[0].ResponseBody)
	}
	if list[0].RequestBody != nil {
		t.Errorf("requestBody = %+v, want none for a record that had no request body", list[0].RequestBody)
	}

	text, err := store.ReadBody(ctx, "rec-big", domain.SideResponse)
	if err != nil {
		t.Fatalf("ReadBody: %v", err)
	}
	if len(text) != len(big) {
		t.Errorf("ReadBody returned %d bytes, want %d", len(text), len(big))
	}
	if _, err := store.ReadBody(ctx, "rec-big", domain.SideRequest); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a body that was never saved = %v, want ErrNotFound", err)
	}
}

func TestRecordsFilterAndOrder(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	for i, spec := range []struct {
		id     string
		source domain.RecordSource
	}{{"m-1", domain.SourceManual}, {"b-1", domain.SourceBrowser}, {"m-2", domain.SourceManual}} {
		rec := sampleRecord(spec.id, spec.source)
		rec.StartedAt = int64(1000 + i)
		if err := store.SaveRecord(ctx, rec); err != nil {
			t.Fatalf("SaveRecord: %v", err)
		}
	}

	manual, err := store.Records(ctx, domain.SourceManual, 0)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(manual) != 2 || manual[0].ID != "m-2" || manual[1].ID != "m-1" {
		t.Errorf("manual records = %v, want newest first", ids(manual))
	}

	browser, err := store.Records(ctx, domain.SourceBrowser, 0)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(browser) != 1 || browser[0].ID != "b-1" {
		t.Errorf("browser records = %v", ids(browser))
	}

	// A limit keeps the newest, which is what the list asks for.
	limited, err := store.Records(ctx, "", 2)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(limited) != 2 || limited[0].ID != "m-2" {
		t.Errorf("limited = %v, want the two newest", ids(limited))
	}
}

func TestDuplicateRecordIDIsAConflict(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveRecord(ctx, sampleRecord("rec-1", domain.SourceManual)); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}
	err := store.SaveRecord(ctx, sampleRecord("rec-1", domain.SourceManual))
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("second save = %v, want domain.ErrConflict", err)
	}
}

func TestDeleteRecordsTakesTheirBodies(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveRecord(ctx, sampleRecord("rec-1", domain.SourceManual)); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}
	if err := store.SaveRecord(ctx, sampleRecord("rec-2", domain.SourceBrowser)); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}

	if err := store.DeleteRecords(ctx, []string{"rec-1"}); err != nil {
		t.Fatalf("DeleteRecords: %v", err)
	}

	// The cascade is what proves the foreign key is enforced on the connection the pool hands out.
	if _, err := store.ReadBody(ctx, "rec-1", domain.SideResponse); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the body outlived its record: %v", err)
	}
	list, _ := store.Records(ctx, "", 0)
	if len(list) != 1 || list[0].ID != "rec-2" {
		t.Errorf("records = %v, want only the survivor", ids(list))
	}
}

func TestPruneByCountAndAge(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 5; i++ {
		rec := sampleRecord("rec-"+string(rune('a'+i)), domain.SourceManual)
		// One of them is old enough to fall out of a week-long window.
		rec.StartedAt = now.Add(-time.Duration(i) * 24 * time.Hour).UnixMilli()
		if err := store.SaveRecord(ctx, rec); err != nil {
			t.Fatalf("SaveRecord: %v", err)
		}
	}

	removed, err := store.Prune(ctx, domain.PruneOptions{MaxCount: 3})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if removed != 2 {
		t.Errorf("pruned %d by count, want 2", removed)
	}
	list, _ := store.Records(ctx, "", 0)
	if len(list) != 3 {
		t.Fatalf("records = %d, want the newest three", len(list))
	}

	removed, err = store.Prune(ctx, domain.PruneOptions{MaxAge: 48 * time.Hour})
	if err != nil {
		t.Fatalf("Prune by age: %v", err)
	}
	if removed != 1 {
		t.Errorf("pruned %d by age, want the one older than two days", removed)
	}
	list, _ = store.Records(ctx, "", 0)
	if len(list) != 2 {
		t.Errorf("records = %d, want two left", len(list))
	}
}

func ids(rows []domain.Record) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.ID
	}
	return out
}
