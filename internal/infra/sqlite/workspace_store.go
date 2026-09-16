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
		w.Personal = w.ID == domain.WorkspacePersonalID
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
	w.Personal = w.ID == domain.WorkspacePersonalID
	return w, nil
}

// SaveWorkspace writes a workspace's own row. Personal is not a column: it is read back out of the
// id, and writing it down would be a second place for the same answer to live.
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

// ActiveWorkspace is the workspace the window is showing: the stored pointer, or the one the app
// is born with when the pointer is empty, missing, or names a space that has since been deleted.
// It never fails on a bad value — a window with no workspace to draw is not a state the app has.
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
	return domain.WorkspacePersonalID, nil
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
