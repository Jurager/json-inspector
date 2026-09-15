package record

import (
	"context"
	"time"

	"json-inspector/internal/domain"
)

// Ingest stores a record the app was told about and tells the window. The window gets the app's
// own record type — not the extension's shape — so both sources draw the same way.
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

// List is the history panel's read: the newest records of one source, or of both when it is
// empty. Bodies stay behind, so switching records costs nothing until one is opened.
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

// Record is one record by id, bodies included — what a run's row opens.
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

// A body the sender had to cut short says so: the window then offers to fetch the rest rather
// than drawing half a document as if it were all of it.
func bodyRef(text string, truncated bool) *domain.BodyRef {
	return &domain.BodyRef{Inline: text, Size: int64(len(text)), Truncated: truncated}
}

// millisToMicros keeps zero as "nothing to report": a page that timed no phase must stay
// distinguishable from one that measured a zero.
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
