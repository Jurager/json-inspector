package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Import statuses. A row exists from the moment an import is claimed, so a crash halfway leaves
// `pending` behind and the next launch tries again.
const (
	ImportPending = "pending"
	ImportDone    = "done"
	ImportFailed  = "failed"
)

// ClaimImport records that an import of this source is about to run, and reports whether this run
// is the one that owns it. An import that already finished is never repeated.
func (s *Store) ClaimImport(ctx context.Context, source string) (bool, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM data_imports WHERE source = ?`,
		source).Scan(&status)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO data_imports (source, status, started_at) VALUES (?, ?, ?)`,
			source, ImportPending, time.Now().UnixMilli())
		if err != nil {
			return false, fmt.Errorf("claiming import %s: %w", source, err)
		}
		return true, nil
	case err != nil:
		return false, fmt.Errorf("reading import %s: %w", source, err)
	}
	// A claim that finished is never repeated. One that failed is retried on the next launch: the
	// reason is usually a database that would not open, and the user fixes that by restarting.
	return status == ImportPending || status == ImportFailed, nil
}

func (s *Store) FinishImport(ctx context.Context, source, status, detail string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE data_imports SET status = ?, finished_at = ?, detail = ? WHERE source = ?`,
		status, time.Now().UnixMilli(), detail, source)
	if err != nil {
		return fmt.Errorf("finishing import %s: %w", source, err)
	}
	return nil
}
