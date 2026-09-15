package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"json-inspector/internal/domain"
)

// Renumbering the whole level rather than shifting a range is deliberate: a level is a few dozen
// rows, and requests and collections inside it share one number line, which one pass settles and a
// range shift has to get right twice.
//
// Where the row sits and what the level contains are the store's business: the window says what it
// dropped and where, and nothing else about it travels.

// The two tables are one list on screen; which one a row came from is only needed to write it back.
type child struct {
	id           string
	isCollection bool
}

// MoveNode puts a request at a place in a collection: the drop index counts the collection's own
// requests and the collections inside it, in the order they are drawn.
func (s *Store) MoveNode(
	ctx context.Context,
	workspaceID, id string,
	collectionID string,
	position int64,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("moving node %s: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	var from string
	err = tx.QueryRowContext(ctx, `SELECT collection_id FROM collection_nodes WHERE id = ?`,
		id).Scan(&from)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("node %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("moving node %s: %w", id, err)
	}

	// The row joins its new collection first, so that the level it is about to be numbered in already
	// has it: the numbering pass places it by the index it was dropped at.
	if _, err := tx.ExecContext(ctx,
		`UPDATE collection_nodes SET collection_id = ? WHERE id = ?`, collectionID, id); err != nil {
		return fmt.Errorf("moving node %s: %w", id, err)
	}
	if from != collectionID {
		if err := s.renumber(ctx, tx, workspaceID, from, child{}, 0); err != nil {
			return err
		}
	}
	if err := s.renumber(ctx, tx, workspaceID, collectionID, child{id: id}, position); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("moving node %s: %w", id, err)
	}
	return nil
}

// MoveCollection puts a collection inside another, or at the top level when the parent is empty.
// A collection is a row of its own, so its place is its parent and nothing else about it moves.
func (s *Store) MoveCollection(
	ctx context.Context,
	workspaceID, id string,
	parentID string,
	position int64,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("moving collection %s: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	var from string
	err = tx.QueryRowContext(ctx,
		`SELECT ifnull(parent_id, '') FROM collections WHERE id = ?`, id).Scan(&from)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("collection %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("moving collection %s: %w", id, err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE collections SET parent_id = ? WHERE id = ?`, nullIfEmpty(parentID), id); err != nil {
		return fmt.Errorf("moving collection %s: %w", id, err)
	}
	if from != parentID {
		if err := s.renumber(ctx, tx, workspaceID, from, child{}, 0); err != nil {
			return err
		}
	}
	if err := s.renumber(ctx, tx, workspaceID, parentID, child{id: id, isCollection: true},
		position); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("moving collection %s: %w", id, err)
	}
	return nil
}

// The empty id is the top level, and the top level is the workspace's own: without the workspace
// named here, numbering one space's roots would rewrite every other space's positions.
func level(ctx context.Context, tx *sql.Tx, workspaceID, collectionID string) ([]child, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, 0 AS is_collection, position FROM collection_nodes WHERE collection_id = ?
		  UNION ALL
		 SELECT id, 1, position FROM collections
		  WHERE workspace_id = ? AND ifnull(parent_id, '') = ?
		  ORDER BY position, id`, collectionID, workspaceID, collectionID)
	if err != nil {
		return nil, fmt.Errorf("reading the level of %s: %w", collectionID, err)
	}
	defer rows.Close()

	out := []child{}
	for rows.Next() {
		var (
			one      child
			position int64
		)
		if err := rows.Scan(&one.id, &one.isCollection, &position); err != nil {
			return nil, fmt.Errorf("reading the level of %s: %w", collectionID, err)
		}
		out = append(out, one)
	}
	return out, rows.Err()
}

// A zero moved id numbers the level as it stands, which is what the level a row left behind needs;
// a named one is lifted out and put back at the drop index, so the index means the same on both
// sides.
func (s *Store) renumber(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID, collectionID string,
	moved child,
	at int64,
) error {
	levelRows, err := level(ctx, tx, workspaceID, collectionID)
	if err != nil {
		return err
	}
	if moved.id != "" {
		levelRows = placeAt(levelRows, moved, at)
	}

	for i, one := range levelRows {
		table := "collection_nodes"
		if one.isCollection {
			table = "collections"
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE `+table+` SET position = ? WHERE id = ?`, int64(i), one.id); err != nil {
			return fmt.Errorf("numbering %s in %s: %w", one.id, collectionID, err)
		}
	}
	return nil
}

// The index counts the level as it looks now, the moving row included, so "after the row at n" is
// one expression — n+1 — in the level it left and the one it joined; the row is discounted here or
// it would be counted twice and land one place too far.
func placeAt(level []child, moved child, at int64) []child {
	others := make([]child, 0, len(level))
	insert := int64(0)
	for i, one := range level {
		if one.id == moved.id {
			continue
		}
		if int64(i) < at {
			insert++
		}
		others = append(others, one)
	}

	// A row dropped below the last one belongs after it, not nowhere: an index past the end, or a
	// negative one — which no row can be dropped at, but which a caller may still send — lands at the
	// nearest end.
	if insert > int64(len(others)) {
		insert = int64(len(others))
	}
	if insert < 0 {
		insert = 0
	}

	out := make([]child, 0, len(others)+1)
	out = append(out, others[:insert]...)
	out = append(out, moved)
	out = append(out, others[insert:]...)
	return out
}
