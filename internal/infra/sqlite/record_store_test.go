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
			DurationUs: 42000,
			StartedAt:  time.Now().UnixMilli(),
			TabID:      7,
			TabTitle:   "Example",
		},
		DNSUs:      micros(1000),
		ConnectUs:  micros(2000),
		TLSUs:      micros(3000),
		WaitUs:     micros(4000),
		DownloadUs: micros(5000),
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

// micros is a phase length for the fixtures: the field is a pointer, because a phase that did not
// happen is not the same as one that took no time.
func micros(us int64) *int64 { return &us }

func TestRecordRoundTrip(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	saved := sampleRecord("rec-1", domain.SourceManual)
	saved.ResponseBytes = 11
	if err := store.SaveRecord(ctx, ws, saved); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}

	list, err := store.Records(ctx, ws, domain.SourceManual, 0)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("records = %d, want one", len(list))
	}
	row := list[0]
	if row.Method != "GET" || row.Status != 200 || row.TabTitle != "Example" {
		t.Errorf("record = %+v, want the saved fields", row)
	}
	if row.WaitUs == nil || *row.WaitUs != 4000 || row.DurationUs != 42000 || row.ResponseBytes != 11 {
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
	if err := store.SaveRecord(ctx, ws, rec); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}

	list, err := store.Records(ctx, ws, "", 0)
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

// One record by id, read the way the list reads a page of them: a run's row opens the record it
// produced, and the viewer then asks for the body that did not travel with it.
func TestRecordReadsOneByID(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveRecord(ctx, ws, domain.Record{
		RecordSummary: domain.RecordSummary{
			ID: "rec-1", Source: domain.SourceManual, Method: "GET", URL: "https://api.example.com/users",
			Status: 200, StatusText: "200 OK",
		},
		RequestHeaders:  []domain.HeaderPair{{Name: "Accept", Value: "application/vnd.api+json"}},
		ResponseHeaders: []domain.HeaderPair{{Name: "Content-Type", Value: "application/json"}},
		RequestBody:     &domain.BodyRef{Inline: `{"a": 1}`, Size: 8},
		ResponseBody:    &domain.BodyRef{Inline: `{"data": []}`, Size: 12},
	}); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}

	rec, err := store.Record(ctx, "rec-1")
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if rec.URL != "https://api.example.com/users" || rec.Status != 200 || rec.Method != "GET" {
		t.Errorf("record = %+v, want the saved one", rec.RecordSummary)
	}
	if len(rec.RequestHeaders) != 1 || len(rec.ResponseHeaders) != 1 {
		t.Errorf("record = %+v, want its headers", rec)
	}
	// The bodies come as references and not as text, for the reason the list does the same: the viewer
	// asks for a body only once it is on screen.
	if rec.RequestBody == nil || rec.RequestBody.Size != 8 || rec.RequestBody.Inline != "" {
		t.Errorf("request body = %+v, want a reference to it", rec.RequestBody)
	}
	if rec.ResponseBody == nil || rec.ResponseBody.Size != 12 {
		t.Errorf("response body = %+v, want a reference to it", rec.ResponseBody)
	}

	if _, err := store.Record(ctx, "нет-такой"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a record that does not exist = %v, want ErrNotFound", err)
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
		if err := store.SaveRecord(ctx, ws, rec); err != nil {
			t.Fatalf("SaveRecord: %v", err)
		}
	}

	manual, err := store.Records(ctx, ws, domain.SourceManual, 0)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(manual) != 2 || manual[0].ID != "m-2" || manual[1].ID != "m-1" {
		t.Errorf("manual records = %v, want newest first", ids(manual))
	}

	browser, err := store.Records(ctx, ws, domain.SourceBrowser, 0)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(browser) != 1 || browser[0].ID != "b-1" {
		t.Errorf("browser records = %v", ids(browser))
	}

	// A limit keeps the newest, which is what the list asks for.
	limited, err := store.Records(ctx, ws, "", 2)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(limited) != 2 || limited[0].ID != "m-2" {
		t.Errorf("limited = %v, want the two newest", ids(limited))
	}
}

// A phase that did not happen stays absent through the database: a fact about the attempt, not a
// zero to be averaged.
func TestAbsentPhasesStayAbsent(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	rec := sampleRecord("rec-nodial", domain.SourceManual)
	rec.DNSUs, rec.ConnectUs, rec.TLSUs = nil, nil, nil
	if err := store.SaveRecord(ctx, ws, rec); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}

	list, err := store.Records(ctx, ws, "", 0)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	got := list[0]
	if got.DNSUs != nil || got.ConnectUs != nil || got.TLSUs != nil {
		t.Errorf("dial phases = %v/%v/%v, want them absent — nothing was dialled",
			got.DNSUs, got.ConnectUs, got.TLSUs)
	}
	if got.WaitUs == nil || *got.WaitUs != 4000 {
		t.Errorf("waitUs = %v, want the phase that did happen", got.WaitUs)
	}
}

func TestDuplicateRecordIDIsAConflict(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveRecord(ctx, ws, sampleRecord("rec-1", domain.SourceManual)); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}
	err := store.SaveRecord(ctx, ws, sampleRecord("rec-1", domain.SourceManual))
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("second save = %v, want domain.ErrConflict", err)
	}
}

func TestDeleteRecordsTakesTheirBodies(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveRecord(ctx, ws, sampleRecord("rec-1", domain.SourceManual)); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}
	if err := store.SaveRecord(ctx, ws, sampleRecord("rec-2", domain.SourceBrowser)); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}

	if err := store.DeleteRecords(ctx, []string{"rec-1"}); err != nil {
		t.Fatalf("DeleteRecords: %v", err)
	}

	// The cascade is what proves the foreign key is enforced on the connection the pool hands out.
	if _, err := store.ReadBody(ctx, "rec-1", domain.SideResponse); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the body outlived its record: %v", err)
	}
	list, _ := store.Records(ctx, ws, "", 0)
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
		// The ages are nudged past the whole days on purpose: the record the age window is meant to
		// drop would otherwise sit exactly on the boundary, and the test would race the clock.
		rec.StartedAt = now.Add(-time.Duration(i)*24*time.Hour - 90*time.Minute).UnixMilli()
		if err := store.SaveRecord(ctx, ws, rec); err != nil {
			t.Fatalf("SaveRecord: %v", err)
		}
	}

	removed, err := store.Prune(ctx, ws, domain.PruneOptions{MaxCount: 3})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if removed != 2 {
		t.Errorf("pruned %d by count, want 2", removed)
	}
	list, _ := store.Records(ctx, ws, "", 0)
	if len(list) != 3 {
		t.Fatalf("records = %d, want the newest three", len(list))
	}

	removed, err = store.Prune(ctx, ws, domain.PruneOptions{MaxAge: 48 * time.Hour})
	if err != nil {
		t.Fatalf("Prune by age: %v", err)
	}
	if removed != 1 {
		t.Errorf("pruned %d by age, want the one older than two days", removed)
	}
	list, _ = store.Records(ctx, ws, "", 0)
	if len(list) != 2 {
		t.Errorf("records = %d, want two left", len(list))
	}
}

// The count a retention rule holds to is spent inside the space it is applied to. Both spaces below
// hold the same three old records, and the prune of one has to leave the other exactly as it was —
// otherwise a busy space would eat the history of the quiet one beside it.
func TestPruneTakesOnlyItsOwnWorkspace(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTeam(t, store)

	for _, id := range []string{"a", "b", "c"} {
		mine := sampleRecord("rec-mine-"+id, domain.SourceManual)
		mine.StartedAt = time.Now().Add(-time.Hour).UnixMilli()
		if err := store.SaveRecord(ctx, ws, mine); err != nil {
			t.Fatalf("SaveRecord: %v", err)
		}
		theirs := sampleRecord("rec-team-"+id, domain.SourceManual)
		theirs.StartedAt = time.Now().Add(-time.Hour).UnixMilli()
		if err := store.SaveRecord(ctx, team, theirs); err != nil {
			t.Fatalf("SaveRecord(%s): %v", team, err)
		}
	}

	removed, err := store.Prune(ctx, ws, domain.PruneOptions{MaxCount: 1})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if removed != 2 {
		t.Errorf("pruned %d, want the two the personal space was over the count by", removed)
	}

	list, err := store.Records(ctx, ws, "", 10)
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("the personal space holds %d record(s), want the one it was cut down to", len(list))
	}

	theirs, err := store.Records(ctx, team, "", 10)
	if err != nil {
		t.Fatalf("Records(%s): %v", team, err)
	}
	if len(theirs) != 3 {
		t.Errorf("the space beside it holds %d record(s), want all three untouched", len(theirs))
	}
}

func ids(rows []domain.Record) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.ID
	}
	return out
}
