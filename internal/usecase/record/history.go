package record

import (
	"context"
	"time"

	"json-inspector/internal/domain"
)

// What history holds and how it is read back: the list without its bodies, one record whole,
// and the body a viewer asks for on its own.

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
		// A body that came in from outside is never marked short: nothing in what arrived says it was
		// cut, and guessing would be worse than taking it whole.
		RequestBody:   bodyRef(in.RequestBody, false),
		ResponseBody:  bodyRef(in.ResponseBody, false),
		RequestBytes:  int64(len(in.RequestBody)),
		ResponseBytes: int64(len(in.ResponseBody)),
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
func (u *UseCase) List(
	ctx context.Context,
	source domain.RecordSource,
	limit int,
) ([]domain.Record, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = KeepCount
	}
	return u.store.Records(ctx, workspace, source, limit)
}

// Record is one record by id — what a run's row opens: the run knows which record it produced, and
// the row says which endpoint it was, but what it answered lives here.
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

// bodyRef is the way in for a body: the whole text, with the size that lets the store decide how
// much of it travels back out. A body the sender had to cut short says so — the window then offers
// to fetch the rest rather than drawing half a document as if it were all of it.
func bodyRef(text string, truncated bool) *domain.BodyRef {
	return &domain.BodyRef{Inline: text, Size: int64(len(text)), Truncated: truncated}
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
