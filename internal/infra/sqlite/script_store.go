package sqlite

// What the scripting feature keeps: the code of every level, and the report of every run.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"json-inspector/internal/domain"
)

// encodeScripts keeps "not set here" apart from "nothing to run": the column is NULL for the first
// and a JSON object for the second, which is the difference between inheriting and having nothing
// to add. An empty object is still a value, and it is written as one.
func encodeScripts(scripts *domain.Scripts) (sql.NullString, error) {
	if scripts == nil {
		return sql.NullString{}, nil
	}
	encoded, err := json.Marshal(scripts)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("encoding the scripts: %w", err)
	}
	return sql.NullString{String: string(encoded), Valid: true}, nil
}

// Scripts reads what is set on a level — a collection, a node, or the draft the command line is
// composing — and nothing when nothing is set there: the editor shows the difference between "take
// the parent's" and "nothing to run here".
//
// The workspace is named because one of the three tables needs it: the command line's draft is the
// same fixed key in every workspace, so without it this query would read a level of the wrong space
// — whichever row the plan happened to reach first.
func (s *Store) Scripts(ctx context.Context, workspaceID, id string) (*domain.Scripts, error) {
	var raw sql.NullString
	// Three tables and one id space: a level is a collection, a node of one, or the draft the command
	// line is composing, and whoever asks knows the id and not the table.
	err := s.db.QueryRowContext(ctx,
		`SELECT scripts_json FROM collections WHERE id = ?
		 UNION ALL SELECT scripts_json FROM collection_nodes WHERE id = ?
		 UNION ALL SELECT scripts_json FROM drafts WHERE workspace_id = ? AND id = ?`,
		id, id, workspaceID, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("level %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("reading the scripts of %s: %w", id, err)
	}
	if !raw.Valid {
		return nil, nil
	}
	var scripts domain.Scripts
	if err := json.Unmarshal([]byte(raw.String), &scripts); err != nil {
		return nil, fmt.Errorf("reading the scripts of %s: %w", id, err)
	}
	return &scripts, nil
}

// SaveScripts writes what is set on a level. NULL stays NULL: a level with no code of its own is a
// level that inherits, and writing an empty object there would turn "take the parents" into
// "nothing to run" — the two the column exists to tell apart.
func (s *Store) SaveScripts(
	ctx context.Context,
	workspaceID, id string,
	scripts *domain.Scripts,
) error {
	encoded, err := encodeScripts(scripts)
	if err != nil {
		return err
	}
	// The draft is the one of the three whose id is the same word in every workspace, so it is the
	// one that has to say which workspace it means: without it, saving the command line's code would
	// write it into every workspace's command line.
	levels := []struct {
		table string
		where string
		args  []any
	}{
		{"collections", `id = ?`, []any{id}},
		{"collection_nodes", `id = ?`, []any{id}},
		{"drafts", `workspace_id = ? AND id = ?`, []any{workspaceID, id}},
	}
	for _, level := range levels {
		result, err := s.db.ExecContext(ctx,
			`UPDATE `+level.table+` SET scripts_json = ?, updated_at = ? WHERE `+level.where,
			append([]any{encoded, time.Now().UnixMilli()}, level.args...)...)
		if err != nil {
			return fmt.Errorf("saving the scripts of %s: %w", id, err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("saving the scripts of %s: %w", id, err)
		}
		if changed > 0 {
			return nil
		}
	}
	return fmt.Errorf("level %s: %w", id, domain.ErrNotFound)
}

// SaveScriptRun writes one execution of one script with everything it printed and asserted. It is
// one transaction because it is one report: a run whose lines were only half written would lie
// about what happened, and the tab draws exactly this.
func (s *Store) SaveScriptRun(ctx context.Context, workspaceID string, run domain.ScriptRun) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("saving run %s: %w", run.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	// The record is named by the window, which addresses records by id; what the column holds is the
	// row the body tables hang off. A record that does not exist leaves the run without one rather
	// than failing the write.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO script_runs (id, workspace_id, record_seq, node_id, scope, ok, error, duration_us,
		                         created_at)
		 VALUES (?, ?, (SELECT seq FROM records WHERE id = ?), ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   ok = excluded.ok, error = excluded.error, duration_us = excluded.duration_us`,
		run.ID, workspaceID, run.RecordID, run.NodeID, string(run.Scope), run.OK, run.Error,
		run.DurationUs, run.CreatedAt); err != nil {
		return fmt.Errorf("saving run %s: %w", run.ID, err)
	}

	// Written whole and written again whole: a save that replaces a run replaces what it said.
	if _, err := tx.ExecContext(ctx, `DELETE FROM script_logs WHERE run_id = ?`, run.ID); err != nil {
		return fmt.Errorf("saving run %s: %w", run.ID, err)
	}
	for i, line := range run.Logs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO script_logs (run_id, position, level, message) VALUES (?, ?, ?, ?)`,
			run.ID, i, line.Level, line.Message); err != nil {
			return fmt.Errorf("saving run %s: %w", run.ID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM test_results WHERE run_id = ?`, run.ID); err != nil {
		return fmt.Errorf("saving run %s: %w", run.ID, err)
	}
	for i, test := range run.Tests {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO test_results (run_id, position, name, passed, error, duration_us)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			run.ID, i, test.Name, test.Passed, test.Error, test.DurationUs); err != nil {
			return fmt.Errorf("saving run %s: %w", run.ID, err)
		}
	}
	return tx.Commit()
}

// ScriptRuns reads what the scripts of one record did, in the order they ran — the collection's
// first, then the folders', then the request's own, which is the order they were executed in.
func (s *Store) ScriptRuns(ctx context.Context, recordID string) ([]domain.ScriptRun, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, node_id, scope, ok, error, duration_us, created_at
		   FROM script_runs
		  WHERE record_seq = (SELECT seq FROM records WHERE id = ?)
		  ORDER BY created_at, rowid`, recordID)
	if err != nil {
		return nil, fmt.Errorf("reading the runs of record %s: %w", recordID, err)
	}
	defer rows.Close()

	runs := []domain.ScriptRun{}
	byID := map[string]int{}
	for rows.Next() {
		run := domain.ScriptRun{RecordID: recordID, Logs: []domain.ScriptLog{},
			Tests: []domain.TestResult{}}
		var nodeID sql.NullString
		if err := rows.Scan(&run.ID, &nodeID, &run.Scope, &run.OK, &run.Error,
			&run.DurationUs, &run.CreatedAt); err != nil {
			return nil, fmt.Errorf("reading the runs of record %s: %w", recordID, err)
		}
		run.NodeID = nodeID.String
		byID[run.ID] = len(runs)
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading the runs of record %s: %w", recordID, err)
	}
	if len(runs) == 0 {
		return runs, nil
	}

	if err := s.attachLogs(ctx, runs, byID); err != nil {
		return nil, err
	}
	if err := s.attachTests(ctx, runs, byID); err != nil {
		return nil, err
	}
	return runs, nil
}

// attachLogs reads every line of every run in one query and hangs them where they belong: two runs
// are two rows of the same report, and the tab draws them together.
func (s *Store) attachLogs(
	ctx context.Context,
	runs []domain.ScriptRun,
	byID map[string]int,
) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT run_id, level, message
		   FROM script_logs
		  WHERE run_id IN (SELECT id FROM script_runs
		                    WHERE record_seq = (SELECT seq FROM records WHERE id = ?))
		  ORDER BY run_id, position`, runs[0].RecordID)
	if err != nil {
		return fmt.Errorf("reading the logs of record %s: %w", runs[0].RecordID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var runID string
		line := domain.ScriptLog{}
		if err := rows.Scan(&runID, &line.Level, &line.Message); err != nil {
			return fmt.Errorf("reading the logs of record %s: %w", runs[0].RecordID, err)
		}
		if at, ok := byID[runID]; ok {
			runs[at].Logs = append(runs[at].Logs, line)
		}
	}
	return rows.Err()
}

func (s *Store) attachTests(
	ctx context.Context,
	runs []domain.ScriptRun,
	byID map[string]int,
) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT run_id, name, passed, error, duration_us
		   FROM test_results
		  WHERE run_id IN (SELECT id FROM script_runs
		                    WHERE record_seq = (SELECT seq FROM records WHERE id = ?))
		  ORDER BY run_id, position`, runs[0].RecordID)
	if err != nil {
		return fmt.Errorf("reading the tests of record %s: %w", runs[0].RecordID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var runID string
		test := domain.TestResult{}
		if err := rows.Scan(&runID, &test.Name, &test.Passed, &test.Error, &test.DurationUs); err != nil {
			return fmt.Errorf("reading the tests of record %s: %w", runs[0].RecordID, err)
		}
		if at, ok := byID[runID]; ok {
			runs[at].Tests = append(runs[at].Tests, test)
		}
	}
	return rows.Err()
}
