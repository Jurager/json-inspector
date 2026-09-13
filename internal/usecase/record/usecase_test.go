package record

import (
	"context"
	"errors"
	"testing"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

type fakeStore struct {
	saved    []domain.Record
	bodies   map[string]string
	pruned   []domain.PruneOptions
	listed   []int
	saveFail error
	imports  map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{bodies: map[string]string{}, imports: map[string]string{}}
}

func (f *fakeStore) SaveRecord(_ context.Context, rec domain.Record) error {
	if f.saveFail != nil {
		return f.saveFail
	}
	for _, existing := range f.saved {
		if existing.ID == rec.ID {
			return domain.ErrConflict
		}
	}
	f.saved = append(f.saved, rec)
	if rec.ResponseBody != nil {
		f.bodies[rec.ID] = rec.ResponseBody.Inline
	}
	return nil
}

func (f *fakeStore) Records(_ context.Context, source domain.RecordSource, limit int) ([]domain.Record, error) {
	f.listed = append(f.listed, limit)
	out := []domain.Record{}
	for _, rec := range f.saved {
		if source != "" && rec.Source != source {
			continue
		}
		out = append(out, rec)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) ReadBody(_ context.Context, id string, _ domain.BodySide) (string, error) {
	body, ok := f.bodies[id]
	if !ok {
		return "", domain.ErrNotFound
	}
	return body, nil
}

func (f *fakeStore) DeleteRecords(_ context.Context, ids []string) error {
	doomed := map[string]bool{}
	for _, id := range ids {
		doomed[id] = true
	}
	kept := f.saved[:0]
	for _, rec := range f.saved {
		if !doomed[rec.ID] {
			kept = append(kept, rec)
		}
	}
	f.saved = kept
	return nil
}

func (f *fakeStore) Prune(_ context.Context, opts domain.PruneOptions) (int, error) {
	f.pruned = append(f.pruned, opts)
	return 0, nil
}

func (f *fakeStore) ClaimImport(_ context.Context, source string) (bool, error) {
	if status, ok := f.imports[source]; ok && status != "pending" {
		return false, nil
	}
	f.imports[source] = "pending"
	return true, nil
}

func (f *fakeStore) FinishImport(_ context.Context, source, status, _ string) error {
	f.imports[source] = status
	return nil
}

type fakeExecutor struct {
	got       []Request
	response  domain.Response
	cancelled []string
	stopEarly bool
}

func (f *fakeExecutor) Execute(_ context.Context, req Request) (*domain.Response, error) {
	f.got = append(f.got, req)
	resp := f.response
	if f.stopEarly {
		resp = domain.Response{Cancelled: true}
	}
	return &resp, nil
}

func (f *fakeExecutor) Cancel(id string) bool {
	f.cancelled = append(f.cancelled, id)
	return true
}

type fakeNotifier struct {
	events chan struct {
		topic   string
		payload any
	}
}

func newFakeNotifier() *fakeNotifier {
	return &fakeNotifier{events: make(chan struct {
		topic   string
		payload any
	}, 16)}
}

func (f *fakeNotifier) Publish(topic string, payload any) {
	f.events <- struct {
		topic   string
		payload any
	}{topic, payload}
}

// waitFor reads until the topic arrives, so an asynchronous send can be checked without sleeping.
func (f *fakeNotifier) waitFor(t *testing.T, topic string) any {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-f.events:
			if event.topic == topic {
				return event.payload
			}
		case <-deadline:
			t.Fatalf("no %q event arrived", topic)
		}
	}
}

func newUseCase() (*UseCase, *fakeStore, *fakeExecutor, *fakeNotifier) {
	store := newFakeStore()
	micros := func(us int64) *int64 { return &us }
	executor := &fakeExecutor{response: domain.Response{
		Status: 200, StatusText: "200 OK", Body: `{"data":[]}`, DurationUs: 12000,
		WaitUs: micros(9000), DownloadUs: micros(3000),
	}}
	notifier := newFakeNotifier()
	retention := RetentionSourceFunc(func(context.Context) (domain.Retention, error) {
		return domain.RetainWeek, nil
	})
	return NewUseCase(store, executor, notifier, retention, platform.NewIDGen()), store, executor, notifier
}

func input() SendInput {
	return SendInput{
		Method:        "GET",
		URL:           "https://api.example.com/articles?token=s3cret",
		Headers:       []domain.HeaderPair{{Name: "Authorization", Value: "Bearer s3cret"}},
		Body:          "",
		MaskedURL:     "https://api.example.com/articles?token=••••",
		MaskedHeaders: []domain.HeaderPair{{Name: "Authorization", Value: "Bearer ••••"}},
		Cookies:       []domain.CookieRow{{Name: "session", Value: "abc"}},
	}
}

func TestSendRecordsTheMaskedRequest(t *testing.T) {
	uc, store, executor, notifier := newUseCase()

	id, err := uc.Send(context.Background(), input())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	finished, ok := notifier.waitFor(t, TopicRequestFinished).(RequestFinished)
	if !ok {
		t.Fatal("the finished event is not a RequestFinished")
	}
	if finished.ID != id {
		t.Errorf("finished %q, want the id Send returned", finished.ID)
	}

	// What the engine got is the real request; what history keeps is the masked one.
	if len(executor.got) != 1 {
		t.Fatalf("the engine was asked %d times, want once", len(executor.got))
	}
	if executor.got[0].URL != input().URL {
		t.Errorf("sent %q, want the unmasked URL", executor.got[0].URL)
	}

	rec := finished.Record
	if rec.URL != input().MaskedURL {
		t.Errorf("recorded url = %q, want the masked one", rec.URL)
	}
	if len(rec.RequestHeaders) != 1 || rec.RequestHeaders[0].Value != "Bearer ••••" {
		t.Errorf("recorded headers = %+v, want the masked ones", rec.RequestHeaders)
	}
	if rec.Status != 200 || rec.Source != domain.SourceManual || rec.DurationUs != 12000 {
		t.Errorf("record = %+v, want a 200 from the request builder", rec.RecordSummary)
	}
	// The phases the engine measured travel as they are, and the ones it did not are absent.
	if rec.WaitUs == nil || *rec.WaitUs != 9000 || rec.DNSUs != nil {
		t.Errorf("phases = wait %v, dns %v; want the measured wait and no dial", rec.WaitUs, rec.DNSUs)
	}
	if rec.ResponseBody == nil || rec.ResponseBody.Inline != `{"data":[]}` {
		t.Errorf("responseBody = %+v, want the text the engine returned", rec.ResponseBody)
	}
	if len(store.saved) != 1 || store.saved[0].ID != id {
		t.Errorf("saved %d records, want the one that finished", len(store.saved))
	}
}

func TestSendReportsAStoreFailure(t *testing.T) {
	uc, store, _, notifier := newUseCase()
	store.saveFail = errors.New("disk is full")

	if _, err := uc.Send(context.Background(), input()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	failed, ok := notifier.waitFor(t, TopicRequestFailed).(RequestFailed)
	if !ok {
		t.Fatal("the failed event is not a RequestFailed")
	}
	if failed.Error == "" {
		t.Error("the failure carried no reason")
	}
}

func TestCancelReachesTheEngine(t *testing.T) {
	uc, _, executor, _ := newUseCase()

	if !uc.Cancel("some-id") {
		t.Error("Cancel reported nothing to stop")
	}
	if len(executor.cancelled) != 1 || executor.cancelled[0] != "some-id" {
		t.Errorf("cancelled = %v, want the id it was given", executor.cancelled)
	}
}

// An attempt the user stopped is not history: nothing came back, and the window that stopped it
// already knows it did.
func TestStoppedAttemptIsNotRecorded(t *testing.T) {
	uc, store, executor, notifier := newUseCase()
	executor.stopEarly = true

	id, err := uc.Send(context.Background(), input())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Nothing is published, so the check is that nothing arrived at all.
	select {
	case event := <-notifier.events:
		t.Fatalf("%s arrived for a stopped attempt (%s)", event.topic, id)
	case <-time.After(50 * time.Millisecond):
	}
	if len(store.saved) != 0 {
		t.Errorf("saved %d records for an attempt that was stopped", len(store.saved))
	}
}

// An ingested record is one this app did not send. Without a source it is the browser's, because
// the extension is the only thing outside the app that reports one.
func TestIngestStoresAndAnnounces(t *testing.T) {
	uc, store, _, notifier := newUseCase()

	rec, err := uc.Ingest(context.Background(), IngestInput{
		Method:          "GET",
		URL:             "https://api.example.com/articles",
		Status:          200,
		StatusText:      "200 OK",
		RequestHeaders:  []domain.HeaderPair{{Name: "Accept", Value: "application/json"}},
		ResponseHeaders: []domain.HeaderPair{{Name: "Set-Cookie", Value: "a=1"}, {Name: "Set-Cookie", Value: "b=2"}},
		ResponseBody:    `{"data":[]}`,
		DurationMs:      12,
		WaitMs:          3,
		TabID:           4,
		TabTitle:        "Example",
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rec.Source != domain.SourceBrowser || rec.TabID != 4 {
		t.Errorf("record = %+v, want a browser record with its tab", rec.RecordSummary)
	}
	if rec.StartedAt == 0 {
		t.Error("a record with no timestamp was not given one")
	}
	// What the browser timed is milliseconds; what is recorded is microseconds, and a phase it did
	// not report stays absent.
	if rec.DurationUs != 12_000 || rec.WaitUs == nil || *rec.WaitUs != 3_000 || rec.DNSUs != nil {
		t.Errorf("timings = %dus, wait %v, dns %v", rec.DurationUs, rec.WaitUs, rec.DNSUs)
	}

	announced, ok := notifier.waitFor(t, TopicRecordAdded).(domain.Record)
	if !ok {
		t.Fatal("the added event did not carry a record")
	}
	if announced.ID != rec.ID {
		t.Errorf("announced %q, stored %q", announced.ID, rec.ID)
	}
	if len(store.saved) != 1 {
		t.Errorf("saved %d records, want one", len(store.saved))
	}
}

func TestListGetAndBody(t *testing.T) {
	uc, _, _, notifier := newUseCase()
	ctx := context.Background()

	if _, err := uc.Send(ctx, input()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	// Send is asynchronous; the record is readable once the send has finished.
	notifier.waitFor(t, TopicRequestFinished)

	rows := mustList(t, uc, domain.SourceManual)
	if len(rows) != 1 || rows[0].Method != "GET" {
		t.Fatalf("rows = %+v, want the sent request", rows)
	}
	// What a list carries is the record without its body text: the id is enough to ask for it.
	if rows[0].ResponseBody == nil || rows[0].ResponseBody.Size != int64(len(`{"data":[]}`)) {
		t.Errorf("responseBody = %+v, want its size and no text", rows[0].ResponseBody)
	}

	body, err := uc.Body(ctx, rows[0].ID, domain.SideResponse)
	if err != nil || body != `{"data":[]}` {
		t.Errorf("Body = %q, %v; want the response text", body, err)
	}

	if err := uc.Clear(ctx, []string{rows[0].ID}); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if rows := mustList(t, uc, ""); len(rows) != 0 {
		t.Errorf("after Clear: %+v", rows)
	}
}

func TestListDefaultsToWhatHistoryKeeps(t *testing.T) {
	uc, store, _, _ := newUseCase()

	if _, err := uc.List(context.Background(), "", 0); err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(store.listed) != 1 || store.listed[0] != KeepCount {
		t.Errorf("limit = %v, want %d", store.listed, KeepCount)
	}
}

// Retention keeps the count that has always applied and adds the age window the settings offer.
func TestPruneUsesTheRetentionWindow(t *testing.T) {
	uc, store, _, _ := newUseCase()

	if _, err := uc.Prune(context.Background()); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(store.pruned) != 1 {
		t.Fatalf("pruned %d times, want once", len(store.pruned))
	}
	opts := store.pruned[0]
	if opts.MaxCount != KeepCount {
		t.Errorf("maxCount = %d, want %d", opts.MaxCount, KeepCount)
	}
	if opts.MaxAge != 7*24*time.Hour {
		t.Errorf("maxAge = %s, want the week the settings name", opts.MaxAge)
	}
}

func TestPruneRunsEverySoManySaves(t *testing.T) {
	uc, store, _, _ := newUseCase()
	ctx := context.Background()

	for i := 0; i < pruneEvery-1; i++ {
		rec := domain.Record{RecordSummary: domain.RecordSummary{ID: "rec-" + string(rune('a'+i))}}
		if err := uc.save(ctx, rec); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	if len(store.pruned) != 0 {
		t.Errorf("pruned %d times before the batch was full", len(store.pruned))
	}

	if err := uc.save(ctx, domain.Record{RecordSummary: domain.RecordSummary{ID: "rec-last"}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(store.pruned) != 1 {
		t.Errorf("pruned %d times, want once at the end of the batch", len(store.pruned))
	}
}

// The oldest rows hold headers as an object of names to values; rows written since headers started
// repeating hold them as pairs. A profile can hold both, so the import reads both.
func TestImportLegacyReadsBothHeaderShapes(t *testing.T) {
	uc, store, _, _ := newUseCase()
	ctx := context.Background()

	payload := `[
	  {"id": "old-1", "method": "GET", "url": "https://a.example.com/x", "status": 200, "source": "manual",
	   "startedAt": 1000, "requestHeaders": {"Accept": "application/json"},
	   "responseHeaders": {"Content-Type": "application/json"}, "responseBody": "{\"data\":[]}"},
	  {"id": "old-2", "method": "POST", "url": "https://b.example.com/y", "status": 201, "source": "browser",
	   "startedAt": 2000, "responseHeaders": [{"name": "X-One", "value": "1"}, {"name": "X-One", "value": "2"}],
	   "tabTitle": "Second"},
	  {"id": "", "method": "GET", "url": "", "startedAt": 3000}
	]`

	report, err := uc.ImportLegacy(ctx, payload)
	if err != nil {
		t.Fatalf("ImportLegacy: %v", err)
	}
	if report.Records != 2 || report.Skipped != 1 || !report.Completed {
		t.Fatalf("report = %+v, want two records and one skipped row", report)
	}

	first := store.saved[0]
	if len(first.RequestHeaders) != 1 || first.RequestHeaders[0].Name != "Accept" {
		t.Errorf("an object of headers came through as %+v", first.RequestHeaders)
	}
	second := store.saved[1]
	if second.Source != domain.SourceBrowser || second.TabTitle != "Second" {
		t.Errorf("second = %+v, want the browser row with its tab", second.RecordSummary)
	}
	var repeated int
	for _, h := range second.ResponseHeaders {
		if h.Name == "X-One" {
			repeated++
		}
	}
	if repeated != 2 {
		t.Errorf("X-One appears %d times, want both values kept", repeated)
	}

	// A second run is a no-op: the claim is what makes the import happen once.
	if again, err := uc.ImportLegacy(ctx, payload); err != nil || again.Records != 0 {
		t.Errorf("second run = %+v, %v; want nothing done", again, err)
	}
}

func mustList(t *testing.T, uc *UseCase, source domain.RecordSource) []domain.Record {
	t.Helper()
	rows, err := uc.List(context.Background(), source, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return rows
}
