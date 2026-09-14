package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Settings returns every stored preference. The set is small and read whole at startup, which is
// why the table is key/value rather than a column per setting.
func (s *Store) Settings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("reading settings: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("reading settings: %w", err)
		}
		out[key] = value
	}
	return out, rows.Err()
}

func (s *Store) Setting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading setting %s: %w", key, err)
	}
	return value, true, nil
}

func (s *Store) SaveSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("saving setting %s: %w", key, err)
	}
	return nil
}

// Which environment a request resolves against is not here: it belongs to a workspace, so it is a
// column of that row (see workspace_store.go). A second copy in this table would be a second answer
// to the same question the moment there are two workspaces.
