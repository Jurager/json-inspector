package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"json-inspector/internal/domain"
)

// A run and the rows it produced. It is a file of its own for the reason the tree's reordering is:
// a run is a thing that happened rather than a part of the collection it happened to, and it is
// read and written in one piece — once when it starts, and again every time a request comes back.

// Written twice: once when the run starts, so its results have a parent, and once when it ends,
// with the counters and the total time.
func (s *Store) SaveRun(ctx context.Context, run domain.CollectionRun) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO collection_runs (id, collection_id, node_id, environment, started_at, finished_at,
		                              passed, failed, duration_us)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   finished_at = excluded.finished_at, passed = excluded.passed,
		   failed = excluded.failed, duration_us = excluded.duration_us`,
		run.ID, run.CollectionID, run.NodeID, run.Environment, run.StartedAt, run.FinishedAt,
		run.Passed, run.Failed, run.DurationUs)
	if err != nil {
		return fmt.Errorf("saving run %s: %w", run.ID, err)
	}
	return nil
}

// AppendRunResult adds one row to a run. Rows go in one at a time because that is how a run makes
// them: a fifty-request run whose window is closed after the tenth keeps the ten.
func (s *Store) AppendRunResult(
	ctx context.Context,
	runID string,
	result domain.CollectionRunResult,
) error {
	failure, err := failureColumn(result.Failure)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO collection_run_results (run_id, node_id, position, status, ok, duration_us, error,
		                                     failure, record_id, assertions_passed, assertions_total)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(run_id, position) DO UPDATE SET
		   status = excluded.status, ok = excluded.ok,
		   duration_us = excluded.duration_us, error = excluded.error,
		   failure = excluded.failure,
		   record_id = excluded.record_id,
		   assertions_passed = excluded.assertions_passed,
		   assertions_total = excluded.assertions_total`,
		runID, result.NodeID, result.Position, result.Status, result.OK, result.DurationUs, result.Error,
		failure, nullIfEmpty(result.RecordID), result.AssertionsPassed, result.AssertionsTotal)
	if err != nil {
		return fmt.Errorf("saving a result of run %s: %w", runID, err)
	}
	return nil
}

// failureColumn is a row's refusal the way the column holds it, and an empty string for a row the
// app did not refuse: a failure nobody wrote a sentence for is the machine's, and the error column
// is where that one already lives.
func failureColumn(failure *domain.Failure) (string, error) {
	if failure == nil {
		return "", nil
	}
	encoded, err := json.Marshal(failure)
	if err != nil {
		return "", fmt.Errorf("encoding a failure: %w", err)
	}
	return string(encoded), nil
}

// LastRun reads the newest run of a node, or of the whole collection when the node is empty. The
// rows come with it in the order they were run: the overview draws them in the tree's order, and
// that is the order the run walked.
func (s *Store) LastRun(
	ctx context.Context,
	collectionID string,
	nodeID string,
) (domain.CollectionRun, bool, error) {
	var run domain.CollectionRun
	err := s.db.QueryRowContext(ctx,
		`SELECT id, collection_id, node_id, environment, started_at, finished_at, passed, failed,
		        duration_us
		   FROM collection_runs
		  WHERE collection_id = ? AND node_id = ?
		  ORDER BY started_at DESC, rowid DESC LIMIT 1`,
		collectionID, nodeID).
		Scan(&run.ID, &run.CollectionID, &run.NodeID, &run.Environment, &run.StartedAt,
			&run.FinishedAt, &run.Passed, &run.Failed, &run.DurationUs)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CollectionRun{}, false, nil
	}
	if err != nil {
		return domain.CollectionRun{}, false, fmt.Errorf("reading the last run of %s: %w", collectionID,
			err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT node_id, position, status, ok, duration_us, error, ifnull(failure, ''),
		        ifnull(record_id, ''), assertions_passed, assertions_total
		   FROM collection_run_results WHERE run_id = ? ORDER BY position`, run.ID)
	if err != nil {
		return domain.CollectionRun{}, false, fmt.Errorf("reading the run %s: %w", run.ID, err)
	}
	defer rows.Close()

	run.Results = []domain.CollectionRunResult{}
	for rows.Next() {
		var (
			result  domain.CollectionRunResult
			status  sql.NullInt64
			failure string
		)
		if err := rows.Scan(&result.NodeID, &result.Position, &status, &result.OK,
			&result.DurationUs, &result.Error, &failure, &result.RecordID, &result.AssertionsPassed,
			&result.AssertionsTotal); err != nil {
			return domain.CollectionRun{}, false, fmt.Errorf("reading the run %s: %w", run.ID, err)
		}
		if status.Valid {
			code := int(status.Int64)
			result.Status = &code
		}
		if failure != "" {
			var refusal domain.Failure
			if err := json.Unmarshal([]byte(failure), &refusal); err != nil {
				return domain.CollectionRun{}, false, fmt.Errorf("reading a failure of run %s: %w",
					run.ID, err)
			}
			result.Failure = &refusal
		}
		run.Results = append(run.Results, result)
	}
	return run, true, rows.Err()
}

// The empty id is the top level. A level's requests and its collections are numbered in one
// sequence, so both tables are asked: a new row lands after what it follows, not at a group's end.
