package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"json-inspector/internal/domain"
)

// Workspaces are the scope every other table hangs off. The table is small and read whole — the
// switcher draws all of them at once — so there is no query beyond the list and the one row.

func (s *Store) Workspaces(ctx context.Context) ([]domain.Workspace, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, kind, color, created_at, updated_at
		   FROM workspaces ORDER BY position, created_at`)
	if err != nil {
		return nil, fmt.Errorf("reading workspaces: %w", err)
	}
	defer rows.Close()

	out := []domain.Workspace{}
	for rows.Next() {
		var w domain.Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Kind, &w.Color, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reading workspaces: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) Workspace(ctx context.Context, id string) (domain.Workspace, error) {
	var w domain.Workspace
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, kind, color, created_at, updated_at FROM workspaces WHERE id = ?`, id).
		Scan(&w.ID, &w.Name, &w.Kind, &w.Color, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Workspace{}, fmt.Errorf("workspace %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("reading workspace %s: %w", id, err)
	}
	return w, nil
}

// SaveWorkspace writes a workspace's own row. Neither the position nor the kind is in the update
// list: the position is what keeps the switcher's order and a save never moves a row, and a kind a
// workspace was made with is not something an edit of its name or colour can change.
func (s *Store) SaveWorkspace(ctx context.Context, w domain.Workspace) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workspaces (id, name, kind, color, position, created_at, updated_at)
		 VALUES (?, ?, ?, ?, (SELECT coalesce(max(position), -1) + 1 FROM workspaces), ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, color = excluded.color, updated_at = excluded.updated_at`,
		w.ID, w.Name, string(w.Kind), w.Color, w.CreatedAt, w.UpdatedAt)
	if err != nil {
		return fmt.Errorf("saving workspace %s: %w", w.ID, err)
	}
	return nil
}

// One statement is enough: every table that belongs to a workspace carries the foreign key, and the
// cascade takes history, collections, environments, drafts and script runs with it. The vacuum is
// what gives the space back — incremental mode only makes rows reusable, and a workspace is the one
// thing here big enough to be worth handing back.
func (s *Store) DeleteWorkspace(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id); err != nil {
		return fmt.Errorf("deleting workspace %s: %w", id, err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA incremental_vacuum(256)`); err != nil {
		return fmt.Errorf("vacuuming after workspace %s: %w", id, err)
	}
	return nil
}

// FirstWorkspace names the workspace that has been there longest: the row the schema writes on a
// fresh installation, and the one every fallback points at now that any row may be deleted. An
// empty table is not a state the app has — the last workspace may not be deleted, which is what
// this error says when it somehow happened.
func (s *Store) FirstWorkspace(ctx context.Context) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM workspaces ORDER BY position, created_at LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("no workspace at all: %w", domain.ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("reading the first workspace: %w", err)
	}
	return id, nil
}

// WorkspaceCounts is what each space holds, counted for the two places the window draws it: the
// switcher's rows and the manager's Contents. It is a query over tables this feature does not own,
// which is why it is a reading beside the list and not a column on the row.
//
// Every row of `collections` counts, folders included: the model calls each one a collection, and a
// folder is one that sits inside another. A run is counted through the collection it belongs to —
// `collection_runs` carries no workspace of its own, and the cascade means a run of a deleted
// collection cannot be left behind.
func (s *Store) Counts(ctx context.Context) (map[string]domain.WorkspaceCounts, error) {
	out := map[string]domain.WorkspaceCounts{}
	// One grouped count per kind of thing, each written into its own field: a space with no rows of
	// a kind has no group at all, and the zero it keeps is the right answer for it.
	for _, counted := range []struct {
		query string
		into  func(*domain.WorkspaceCounts) *int
	}{
		{`SELECT workspace_id, count(*) FROM collections GROUP BY workspace_id`,
			func(c *domain.WorkspaceCounts) *int { return &c.Collections }},
		{`SELECT workspace_id, count(*) FROM environments GROUP BY workspace_id`,
			func(c *domain.WorkspaceCounts) *int { return &c.Environments }},
		{`SELECT c.workspace_id, count(*) FROM collection_runs r
		   JOIN collections c ON c.id = r.collection_id
		  GROUP BY c.workspace_id`,
			func(c *domain.WorkspaceCounts) *int { return &c.Runs }},
	} {
		rows, err := s.db.QueryContext(ctx, counted.query)
		if err != nil {
			return nil, fmt.Errorf("counting what a workspace holds: %w", err)
		}
		for rows.Next() {
			var (
				id    string
				count int
			)
			if err := rows.Scan(&id, &count); err != nil {
				rows.Close()
				return nil, fmt.Errorf("counting what a workspace holds: %w", err)
			}
			held := out[id]
			*counted.into(&held) = count
			out[id] = held
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("counting what a workspace holds: %w", err)
		}
	}
	return out, nil
}

// ActiveWorkspace is the workspace the window is showing: the stored pointer, or the oldest one
// when the pointer is empty, missing, or names a space that has since been deleted. A stale pointer
// is not an error — the window has to land somewhere, and the space that has been there longest is
// the one the app is most likely to still have.
func (s *Store) ActiveWorkspace(ctx context.Context) (string, error) {
	pointed, ok, err := s.Setting(ctx, domain.SettingActiveWorkspace)
	if err != nil {
		return "", err
	}
	if ok && pointed != "" {
		var exists int
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM workspaces WHERE id = ?`, pointed).Scan(&exists)
		if err == nil {
			return pointed, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("reading workspace %s: %w", pointed, err)
		}
	}
	return s.FirstWorkspace(ctx)
}

func (s *Store) SetActiveWorkspace(ctx context.Context, id string) error {
	return s.SaveSetting(ctx, domain.SettingActiveWorkspace, id)
}

// activeEnvironment is the environment the workspace is working in. It lives on the workspace's
// own row rather than beside the app's preferences: two spaces are two working environments, and
// a switch that kept one answer would resolve variables against the other space's.
func (s *Store) activeEnvironment(ctx context.Context, workspaceID string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT active_environment_id FROM workspaces WHERE id = ?`, workspaceID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("workspace %s: %w", workspaceID, domain.ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("reading the environment of %s: %w", workspaceID, err)
	}
	return id, nil
}

func (s *Store) SetActiveEnvironment(ctx context.Context, workspaceID, envID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE workspaces SET active_environment_id = ?, updated_at = ? WHERE id = ?`,
		envID, time.Now().UnixMilli(), workspaceID)
	if err != nil {
		return fmt.Errorf("saving the environment of %s: %w", workspaceID, err)
	}
	return nil
}
