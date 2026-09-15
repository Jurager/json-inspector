// Package record owns the history: what was sent, what came back, and how long any of it is kept.
package record

import (
	"context"
	"fmt"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// Event topics. A record appearing and a send finishing are different things: the first is one the
// app was told about, the second is an answer to something the window asked for. There is no
// "started" topic: the caller gets the id from Send itself, and it is the only caller there is.
const (
	TopicRecordAdded     = "record:added"
	TopicRequestFinished = "request:finished"
	TopicRequestFailed   = "request:failed"
)

// KeepCount is how many records history holds. The design puts the age window in settings; this is
// the count that has always applied, and it stays because a burst of captures has no age to judge.
const KeepCount = 200

// pruneEvery is how often a save is followed by a look at the retention rules: often enough that a
// long session cannot grow without bound, rarely enough that it is not a query per request.
const pruneEvery = 20

type UseCase struct {
	store     Store
	scope     Scope
	executor  Executor
	notifier  Notifier
	retention RetentionSource
	screen    Screener
	mask      Masker
	ids       platform.IDGen

	sincePrune int
}

func NewUseCase(
	store Store,
	scope Scope,
	executor Executor,
	notifier Notifier,
	retention RetentionSource,
	screen Screener,
	mask Masker,
	ids platform.IDGen,
) *UseCase {
	return &UseCase{
		store: store, scope: scope, executor: executor, notifier: notifier, retention: retention,
		screen: screen, mask: mask, ids: ids,
	}
}

// RequestFinished carries the record the attempt produced — the same shape history lists, so the
// window has one type to draw and one place to put it.
type RequestFinished struct {
	ID     string        `json:"id"`
	Record domain.Record `json:"record"`
}

// RequestFailed is an attempt that produced no record at all, which is a failure of this side rather
// than of the network: a request that never reached the server still has a record.
type RequestFailed struct {
	ID    string `json:"id"`
	Error string `json:"error"`
	// Failure is the same news in the window's terms — the code and the values its sentence needs —
	// when the app is the side that refused. An error from the network has none, and the window shows
	// the message beside it.
	Failure *domain.Failure `json:"failure,omitempty"`
}

// SendInput is one attempt. The request in it is ready to go — its variables are already filled in —
// and the masked copy beside it is what history keeps.
//
// The two representations exist because the draft is still the window's: it holds the tokens, so it
// is the one that can say what a secret was. When the draft moves here, so does this.
type SendInput struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers []domain.HeaderPair `json:"headers"`
	Body    string              `json:"body"`
	// BodyKind and the two below it are what the body was rendered FROM. They travel beside the text
	// because a masked copy of a form or a file cannot be made from the text: it has to be rendered
	// again out of what the request is made of.
	BodyKind domain.BodyKind  `json:"bodyKind,omitempty"`
	Form     []domain.FormRow `json:"form,omitempty"`
	BodyFile string           `json:"bodyFile,omitempty"`

	MaskedURL     string              `json:"maskedUrl"`
	MaskedHeaders []domain.HeaderPair `json:"maskedHeaders"`
	MaskedBody    string              `json:"maskedBody"`
	Cookies       []domain.CookieRow  `json:"cookies"`

	// Digest is a credential the request cannot carry and the engine has to: see draft.Prepared.
	Digest *domain.DigestCredentials `json:"-"`

	// Node is the request this came from, when it came from a collection, and Run is the scope the
	// run's own variables live in — a collection run's id, or the send itself for a request sent on
	// its own. Both are what the scripts around the attempt are found by; both are empty for a
	// request composed on the command line, which has nothing above it.
	Node string `json:"node,omitempty"`
	Run  string `json:"run,omitempty"`
}

// Send starts a request and returns its id at once. The answer arrives as an event: it has to outlive
// the call that started it, because it can be cancelled and because a slow endpoint must not hold
// the window's promise open.
func (u *UseCase) Send(ctx context.Context, in SendInput) (string, error) {
	// The workspace is read here and carried into the goroutine as a value: the attempt outlives
	// this call, and asking again down there would write the answer into whatever space the user
	// had switched to by the time it arrived.
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return "", err
	}

	id := u.ids()
	started := time.Now().UnixMilli()
	in.Run = runScope(in.Run, id)

	// The attempt runs on its own context: the caller's ends when the frontend call returns, and a
	// request must not be cancelled by the act of asking for it.
	go func() {
		rec, err := u.attempt(context.Background(), workspace, id, started, in)
		if err != nil {
			u.notifier.Publish(TopicRequestFailed, RequestFailed{
				ID:      id,
				Error:   err.Error(),
				Failure: domain.AsFailure(err),
			})
			return
		}
		if rec.Cancelled {
			// An attempt the user stopped has nothing to keep: history holds what came back, and
			// nothing did. The window stopped it, so it already knows.
			return
		}
		u.notifier.Publish(TopicRequestFinished, RequestFinished{ID: id, Record: rec})
	}()

	return id, nil
}

// SendAndWait is Send for a caller that has to see the answer before it can do the next thing: a
// run of a collection asks for one request, looks at what came back, and moves on. A cancelled
// attempt comes back saying so rather than as an error — it was stopped on purpose, and that is an
// answer.
func (u *UseCase) SendAndWait(ctx context.Context, in SendInput) (domain.Record, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.Record{}, err
	}

	id := u.ids()
	in.Run = runScope(in.Run, id)
	rec, err := u.attempt(ctx, workspace, id, time.Now().UnixMilli(), in)
	if err != nil {
		return domain.Record{}, err
	}
	if rec.Cancelled {
		return rec, nil
	}
	u.notifier.Publish(TopicRequestFinished, RequestFinished{ID: id, Record: rec})
	return rec, nil
}

// attempt runs one request to the end: through the scripts around it, out to the engine, folded into
// a record, and saved. It answers with the record, with a cancelled attempt, or with why there is
// none — a transport failure is the engine's to report inside the response, so an error here is this
// side failing.
func (u *UseCase) attempt(ctx context.Context, workspace, id string, started int64, in SendInput) (domain.Record, error) {
	pass := domain.ScriptPass{
		Run:      in.Run,
		RecordID: id,
		NodeID:   in.Node,
		Request: &domain.ScriptRequest{
			Method:   in.Method,
			URL:      in.URL,
			Headers:  in.Headers,
			Body:     in.Body,
			BodyKind: in.BodyKind,
		},
	}

	if u.screen != nil {
		skip, err := u.screen.Before(ctx, workspace, &pass)
		if err != nil {
			return domain.Record{}, err
		}
		if skip {
			// Nothing goes out, so there is no answer to fold in and nothing for history to keep.
			// What is left is the shape of the attempt — which request it was, and that it was not
			// sent — so whoever asked learns that rather than that a request failed.
			return domain.Record{
				RecordSummary: domain.RecordSummary{
					ID:        id,
					Source:    domain.SourceManual,
					Method:    pass.Request.Method,
					URL:       in.MaskedURL,
					StartedAt: started,
				},
				Skipped: true,
			}, nil
		}
	}

	resp, err := u.executor.Execute(ctx, Request{
		ID:      id,
		Method:  pass.Request.Method,
		URL:     pass.Request.URL,
		Headers: pass.Request.Headers,
		Body:    pass.Request.Body,
		Digest:  in.Digest,
	})
	if err != nil {
		return domain.Record{}, err
	}
	if resp.Cancelled {
		return domain.Record{Cancelled: true}, nil
	}

	rec := u.recordFrom(id, workspace, started, in, pass, u.masked(ctx, in, pass.Request), resp)
	if err := u.save(ctx, workspace, rec); err != nil {
		return domain.Record{}, err
	}

	// The second half of the pass runs after the record is written and not before it: its reports hang
	// off that record, and the reports of the first half are carried in the pass for the same reason.
	if u.screen != nil {
		pass.Response = resp
		u.screen.After(ctx, workspace, pass)
	}
	return rec, nil
}

// masked is the request as history keeps it. What the caller prepared is the answer until a script
// changes the request — the mask is made where the values are, and that is not here.
//
// A mask that cannot be made leaves the prepared one in place: the unusable answer is the masked one,
// because the other would be a secret written down in the clear.
func (u *UseCase) masked(ctx context.Context, in SendInput, sent *domain.ScriptRequest) Masked {
	prepared := Masked{URL: in.MaskedURL, Headers: in.MaskedHeaders, Body: in.MaskedBody}
	if u.mask == nil || !changed(in, sent) {
		return prepared
	}
	again, err := u.mask.Mask(ctx, in, *sent)
	if err != nil {
		return prepared
	}
	return again
}

// runScope is where this request's `pm.variables` live: the run it belongs to, or — for a request
// sent on its own, from the command line or from a card — the send itself. Every request has a scope;
// only a run shares one between requests.
func runScope(run string, id string) string {
	if run != "" {
		return run
	}
	return id
}

// changed says whether the scripts made the request something else than what was prepared.
func changed(in SendInput, sent *domain.ScriptRequest) bool {
	if in.URL != sent.URL || in.Body != sent.Body || len(in.Headers) != len(sent.Headers) {
		return true
	}
	for i, header := range in.Headers {
		if sent.Headers[i] != header {
			return true
		}
	}
	return false
}

// Cancel stops an attempt by id. It reports whether anything was still running under it.
func (u *UseCase) Cancel(id string) bool {
	return u.executor.Cancel(id)
}

// sentHeaders is what the request carried, as the record keeps it: the masked rows it was prepared
// with, and whatever the engine itself had to add to get it through.
//
// A Digest answer is the case: it is computed from a challenge that had not arrived when the request
// was prepared, so there is no row anywhere above the engine that could have known it — and a record
// without it would be a record of the request that was refused rather than of the one that answered.
func sentHeaders(masked []domain.HeaderPair, resp *domain.Response) []domain.HeaderPair {
	return append(orEmptyPairs(masked), resp.SentHeaders...)
}

// recordFrom folds a response into the record history keeps: the request that went out, as it can be
// shown, on one side, and what came back on the other.
func (u *UseCase) recordFrom(
	id string,
	workspace string,
	started int64,
	in SendInput,
	pass domain.ScriptPass,
	masked Masked,
	resp *domain.Response,
) domain.Record {
	return domain.Record{
		RecordSummary: domain.RecordSummary{
			ID:          id,
			WorkspaceID: workspace,
			Source:      domain.SourceManual,
			Method:      pass.Request.Method,
			URL:         masked.URL,
			Status:      resp.Status,
			StatusText:  resp.StatusText,
			ContentType: resp.ContentType,
			Error:       resp.Error,
			DurationUs:  resp.DurationUs,
			StartedAt:   started,
		},
		Cancelled:       resp.Cancelled,
		DNSUs:           resp.DNSUs,
		ConnectUs:       resp.ConnectUs,
		TLSUs:           resp.TLSUs,
		WaitUs:          resp.WaitUs,
		DownloadUs:      resp.DownloadUs,
		RequestBytes:    int64(len(pass.Request.Body)),
		ResponseBytes:   int64(len(resp.Body)),
		RequestHeaders:  sentHeaders(masked.Headers, resp),
		ResponseHeaders: orEmptyPairs(resp.Headers),
		RequestCookies:  in.Cookies,
		RequestBody:     bodyRef(masked.Body),
		ResponseBody:    bodyRef(resp.Body, resp.BodyTruncated),
	}
}

// IngestInput is a request this app did not send: one the browser made, as the extension reported
// it, or one that never left the machine at all. Source says which, and an empty one means the
// browser — the extension is the only thing that sends one of these from the outside.
type IngestInput struct {
	Source          domain.RecordSource `json:"source"`
	Method          string              `json:"method"`
	URL             string              `json:"url"`
	Status          int                 `json:"status"`
	StatusText      string              `json:"statusText"`
	ContentType     string              `json:"contentType,omitempty"`
	RequestHeaders  []domain.HeaderPair `json:"requestHeaders"`
	ResponseHeaders []domain.HeaderPair `json:"responseHeaders"`
	RequestBody     string              `json:"requestBody"`
	ResponseBody    string              `json:"responseBody"`
	DurationMs      int64               `json:"durationMs"`
	// Everything below only exists for a record that came from a browser tab. `omitempty` is what
	// says so on the wire, so a caller with no tab names none of them instead of spelling out zeros.
	StartedAt  int64  `json:"startedAt,omitempty"`
	WaitMs     int64  `json:"waitMs,omitempty"`
	DownloadMs int64  `json:"downloadMs,omitempty"`
	TabID      int    `json:"tabId,omitempty"`
	TabTitle   string `json:"tabTitle,omitempty"`
	TabURL     string `json:"tabURL,omitempty"`
	FavIconURL string `json:"favIconUrl,omitempty"`
}

// Ingest stores a record the app was told about and tells the window about it. The record is what
// the window receives — not the extension's own shape — so there is one type to draw and one id to
// select, whether a record came from this app or from the browser.
func (u *UseCase) Ingest(ctx context.Context, in IngestInput) (domain.Record, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.Record{}, err
	}

	started := in.StartedAt
	if started == 0 {
		started = time.Now().UnixMilli()
	}

	source := in.Source
	if source == "" {
		source = domain.SourceBrowser
	}

	rec := domain.Record{
		RecordSummary: domain.RecordSummary{
			ID:          u.ids(),
			WorkspaceID: workspace,
			Source:      source,
			Method:      in.Method,
			URL:         in.URL,
			Status:      in.Status,
			StatusText:  in.StatusText,
			ContentType: in.ContentType,
			DurationUs:  in.DurationMs * 1000,
			StartedAt:   started,
			TabID:       in.TabID,
			TabTitle:    in.TabTitle,
			TabURL:      in.TabURL,
			FavIconURL:  in.FavIconURL,
		},
		RequestHeaders:  orEmptyPairs(in.RequestHeaders),
		ResponseHeaders: orEmptyPairs(in.ResponseHeaders),
		RequestBody:     bodyRef(in.RequestBody),
		ResponseBody:    bodyRef(in.ResponseBody),
		RequestBytes:    int64(len(in.RequestBody)),
		ResponseBytes:   int64(len(in.ResponseBody)),
		// The two phases a page can time itself. Absent stays absent: a capture that reported none is
		// a capture with no phases, not one whose phases were zero.
		WaitUs:     millisToMicros(in.WaitMs),
		DownloadUs: millisToMicros(in.DownloadMs),
	}

	if err := u.save(ctx, workspace, rec); err != nil {
		return domain.Record{}, err
	}
	u.notifier.Publish(TopicRecordAdded, rec)
	return rec, nil
}

// List is what the history panel draws: the newest records of one source, or of both when the
// source is empty. Everything but the body text comes with them, so switching between two records
// costs nothing until a body is actually opened.
func (u *UseCase) List(ctx context.Context, source domain.RecordSource, limit int) ([]domain.Record, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = KeepCount
	}
	return u.store.Records(ctx, workspace, source, limit)
}

// Record is one record by id — what a run's row opens: the run knows which record it produced, and the
// row says which endpoint it was, but what it answered lives here.
func (u *UseCase) Record(ctx context.Context, id string) (domain.Record, error) {
	return u.store.Record(ctx, id)
}

// Body is the call a viewer makes for a body that did not travel with the record.
func (u *UseCase) Body(ctx context.Context, id string, side domain.BodySide) (string, error) {
	return u.store.ReadBody(ctx, id, side)
}

// Clear removes records by id; the list's "clear this group" is this call with the ids it shows.
func (u *UseCase) Clear(ctx context.Context, ids []string) error {
	return u.store.DeleteRecords(ctx, ids)
}

// Prune applies the retention rules to the workspace on screen: the count that has always applied,
// and the age window the settings screen offers on top of it. The rules are per workspace, so a
// space nobody has opened in a month keeps what it holds until it is opened again.
func (u *UseCase) Prune(ctx context.Context) (int, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return 0, err
	}
	return u.prune(ctx, workspace)
}

func (u *UseCase) prune(ctx context.Context, workspace string) (int, error) {
	opts := domain.PruneOptions{MaxCount: KeepCount}
	if u.retention != nil {
		retention, err := u.retention.Retention(ctx)
		if err != nil {
			return 0, err
		}
		opts.MaxAge = ageOf(retention)
	}
	return u.store.Prune(ctx, workspace, opts)
}

// save writes a record and, every so often, looks at what the retention rules would drop. Counting
// saves rather than running a timer means an idle app does no work, and a busy one cannot grow past
// the window between two prunes by more than a batch.
func (u *UseCase) save(ctx context.Context, workspace string, rec domain.Record) error {
	if err := u.store.SaveRecord(ctx, workspace, rec); err != nil {
		return err
	}

	u.sincePrune++
	if u.sincePrune < pruneEvery {
		return nil
	}
	u.sincePrune = 0
	if _, err := u.prune(ctx, workspace); err != nil {
		// Retention that fails is retention the next batch tries again; the record itself is safe.
		return fmt.Errorf("applying retention: %w", err)
	}
	return nil
}

func ageOf(retention domain.Retention) time.Duration {
	switch retention {
	case domain.RetainWeek:
		return 7 * 24 * time.Hour
	case domain.RetainMonth:
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}

// bodyRef is the way in for a body: the whole text, with the size that lets the store decide how
// much of it travels back out.
func bodyRef(text string, truncated ...bool) *domain.BodyRef {
	ref := &domain.BodyRef{Inline: text, Size: int64(len(text))}
	if len(truncated) > 0 {
		ref.Truncated = truncated[0]
	}
	return ref
}

// millisToMicros converts what the browser measured, keeping zero as "nothing to report": a page
// that timed no phase sends none, and that has to stay distinguishable from a measured zero.
func millisToMicros(ms int64) *int64 {
	if ms <= 0 {
		return nil
	}
	us := ms * 1000
	return &us
}

func orEmptyPairs(pairs []domain.HeaderPair) []domain.HeaderPair {
	if pairs == nil {
		return []domain.HeaderPair{}
	}
	return pairs
}
