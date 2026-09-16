package record

import (
	"context"
	"log"
	"time"

	"json-inspector/internal/domain"
)

// The count is what has always applied; the age window is the setting on top of it. Both are
// per workspace: a busy space must not eat a quiet one.

// KeepCount is how many records history holds. The design puts the age window in settings; this is
// the count that has always applied, and it stays because a burst of captures has no age to judge.
const KeepCount = 200

// pruneEvery is how often a save is followed by a look at the retention rules: often enough that a
// long session cannot grow without bound, rarely enough that it is not a query per request.
const pruneEvery = 20

// Prune applies the count and the age window to the workspace on screen. The rules are per
// workspace, so a space nobody has opened in a month keeps what it holds until it is opened.
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

	if u.sincePrune.Add(1) < pruneEvery {
		return nil
	}
	u.sincePrune.Store(0)
	if _, err := u.prune(ctx, workspace); err != nil {
		// The record is written and the send is over: this failure belongs to retention alone, and
		// returning it would tell the window that a request which went out had failed. The next batch
		// tries again.
		log.Printf("[record] retention: %v", err)
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
