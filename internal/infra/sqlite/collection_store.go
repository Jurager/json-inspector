package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"json-inspector/internal/domain"
)

// The tree is assembled here rather than in the window: the order rows are drawn in is the order
// this table keeps, and a tree assembled in two places is a tree that can disagree with itself.
func (s *Store) Collections(ctx context.Context, workspaceID string) ([]domain.Collection, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, position, created_at, updated_at, auth_json, ifnull(parent_id, ''),
		        variables_json
		   FROM collections WHERE workspace_id = ? ORDER BY position, created_at`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("listing collections: %w", err)
	}
	defer rows.Close()

	flat := []domain.Collection{}
	for rows.Next() {
		var (
			c         domain.Collection
			auth      sql.NullString
			variables sql.NullString
		)
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Position, &c.CreatedAt, &c.UpdatedAt,
			&auth, &c.ParentID, &variables); err != nil {
			return nil, fmt.Errorf("listing collections: %w", err)
		}
		if auth.Valid {
			var value domain.Auth
			if err := json.Unmarshal([]byte(auth.String), &value); err != nil {
				return nil, fmt.Errorf("reading the auth of collection %s: %w", c.ID, err)
			}
			c.Auth = &value
		}
		// The variables travel with the tree: every request inside asks for them on the way out, and
		// a second query per send would be a query per keystroke of a preview.
		if c.Variables, err = decodeVariables(variables, "collection "+c.ID); err != nil {
			return nil, err
		}
		// Empty rather than nil: a level with nothing in it is drawn as nothing, and the window
		// would have to guard every walk otherwise.
		c.Items = []domain.CollectionNode{}
		c.Children = []domain.Collection{}
		flat = append(flat, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing collections: %w", err)
	}
	if len(flat) == 0 {
		return flat, nil
	}

	nodes, err := s.nodes(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	// Requests are hung on the collection that holds them, which is a lookup rather than a walk: the
	// rows already came in the order the tree keeps.
	byCollection := map[string][]domain.CollectionNode{}
	for _, node := range nodes {
		byCollection[node.CollectionID] = append(byCollection[node.CollectionID], node)
	}
	for i := range flat {
		if items := byCollection[flat[i].ID]; len(items) > 0 {
			flat[i].Items = items
		}
	}
	return nestCollections(flat), nil
}

// The requests and the collections of a level stay apart although they share one number line:
// merging the two lists by position is drawing a row, and that is the window's.
func nestCollections(flat []domain.Collection) []domain.Collection {
	byParent := map[string][]domain.Collection{}
	for _, c := range flat {
		byParent[c.ParentID] = append(byParent[c.ParentID], c)
	}

	var build func(parent string) []domain.Collection
	build = func(parent string) []domain.Collection {
		children := byParent[parent]
		if children == nil {
			return []domain.Collection{}
		}
		for i := range children {
			children[i].Children = build(children[i].ID)
		}
		return children
	}
	return build("")
}

// Node reads one node with everything a request is made of, which is what opening it needs and what
// the tree deliberately left out.
func (s *Store) Node(ctx context.Context, id string) (domain.CollectionNode, error) {
	var (
		node                        domain.CollectionNode
		params, headers, cookies    string
		auth, scripts               sql.NullString
		description                 sql.NullString
		url, body, method, bodyKind sql.NullString
		form, bodyFile              sql.NullString
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, collection_id, name, position, description, auth_json, scripts_json,
		        method, url, params_json, headers_json, body, body_kind, form_json, body_file,
		        cookies_json, created_at, updated_at
		   FROM collection_nodes WHERE id = ?`, id).
		Scan(&node.ID, &node.CollectionID, &node.Name, &node.Position, &description,
			&auth, &scripts, &method, &url, &params, &headers, &body, &bodyKind, &form, &bodyFile,
			&cookies, &node.CreatedAt, &node.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CollectionNode{}, fmt.Errorf("collection node %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.CollectionNode{}, fmt.Errorf("reading collection node %s: %w", id, err)
	}

	node.Description = description.String
	node.Method = method.String
	node.URL = url.String
	node.Body = body.String
	// A node saved before there were kinds holds text, and an unsaved one holds nothing: both are raw.
	node.BodyKind = domain.KindOf(domain.BodyKind(bodyKind.String))
	node.BodyFile = bodyFile.String
	if form.Valid {
		if err := json.Unmarshal([]byte(form.String), &node.Form); err != nil {
			return domain.CollectionNode{}, fmt.Errorf("reading the form of node %s: %w", id, err)
		}
	}
	for _, part := range []struct {
		raw  string
		into any
		what string
	}{
		{params, &node.Params, "params"},
		{headers, &node.Headers, "headers"},
		{cookies, &node.Cookies, "cookies"},
	} {
		if err := json.Unmarshal([]byte(part.raw), part.into); err != nil {
			return domain.CollectionNode{}, fmt.Errorf("reading the %s of node %s: %w", part.what, id, err)
		}
	}
	if err := readAuth(auth, &node.Auth, "node "+id); err != nil {
		return domain.CollectionNode{}, err
	}
	if scripts.Valid {
		var value domain.Scripts
		if err := json.Unmarshal([]byte(scripts.String), &value); err != nil {
			return domain.CollectionNode{}, fmt.Errorf("reading the scripts of node %s: %w", id, err)
		}
		node.Scripts = &value
	}
	return node, nil
}

// readAuth is the auth a row carries. An absent one is not a failure: it is how a level says it has
// none of its own, and the request below it reads that as "take yours".
func readAuth(raw sql.NullString, into **domain.Auth, what string) error {
	if !raw.Valid {
		return nil
	}
	var value domain.Auth
	if err := json.Unmarshal([]byte(raw.String), &value); err != nil {
		return fmt.Errorf("reading the auth of %s: %w", what, err)
	}
	*into = &value
	return nil
}

func (s *Store) nodes(ctx context.Context, workspaceID string) ([]domain.CollectionNode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, collection_id, name, position, method, description, auth_json
		   FROM collection_nodes
		  WHERE collection_id IN (SELECT id FROM collections WHERE workspace_id = ?)
		  ORDER BY collection_id, position, created_at`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("listing collection nodes: %w", err)
	}
	defer rows.Close()

	out := []domain.CollectionNode{}
	for rows.Next() {
		var (
			node        domain.CollectionNode
			method      sql.NullString
			description sql.NullString
			auth        sql.NullString
		)
		if err := rows.Scan(&node.ID, &node.CollectionID, &node.Name, &node.Position,
			&method, &description, &auth); err != nil {
			return nil, fmt.Errorf("listing collection nodes: %w", err)
		}
		node.Method = method.String
		// The description travels with the row: it is the line the overview draws under the title.
		node.Description = description.String
		// Auth travels with it as well, although no row draws it: inheritance is a property of the
		// tree, and a request has to be able to ask what the levels above it answered.
		if err := readAuth(auth, &node.Auth, "node "+node.ID); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}

// LevelRows is the collection page's read: the address each request of one level goes to, and
// nothing else. It is deliberately not the tree's query — that one carries what the panel draws and
// no request payload at all — and it is deliberately not one node at a time, which is what an
// export does, because an export needs the bodies.
func (s *Store) LevelRows(ctx context.Context, collectionID string) ([]domain.LevelRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, method, url FROM collection_nodes
		  WHERE collection_id = ?
		  ORDER BY position, created_at`, collectionID)
	if err != nil {
		return nil, fmt.Errorf("listing the requests of %s: %w", collectionID, err)
	}
	defer rows.Close()

	out := []domain.LevelRow{}
	for rows.Next() {
		var (
			row    domain.LevelRow
			method sql.NullString
			url    sql.NullString
		)
		if err := rows.Scan(&row.ID, &row.Name, &method, &url); err != nil {
			return nil, fmt.Errorf("listing the requests of %s: %w", collectionID, err)
		}
		row.Method = method.String
		row.URL = url.String
		out = append(out, row)
	}
	return out, rows.Err()
}

// The upsert writes parent_id on the insert and leaves it alone on the update, the way a node's
// collection is: a rename saves the row it read, and moving is a gesture of its own.
func (s *Store) SaveCollection(ctx context.Context, workspaceID string, c domain.Collection) error {
	auth, err := encodeAuth(c.Auth)
	if err != nil {
		return fmt.Errorf("saving collection %s: %w", c.ID, err)
	}
	variables, err := encodeVariables(c.Variables)
	if err != nil {
		return fmt.Errorf("saving collection %s: %w", c.ID, err)
	}

	now := time.Now().UnixMilli()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO collections (id, workspace_id, name, description, position, parent_id, auth_json,
		                          variables_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, description = excluded.description,
		   position = excluded.position, auth_json = excluded.auth_json,
		   variables_json = excluded.variables_json,
		   updated_at = excluded.updated_at`,
		c.ID, workspaceID, c.Name, c.Description, c.Position, nullIfEmpty(c.ParentID), auth, variables,
		now, now); err != nil {
		return fmt.Errorf("saving collection %s: %w", c.ID, err)
	}
	return nil
}

// encodeAuth keeps "not set here" apart from "explicitly nothing": the column is NULL for the
// first and a JSON object for the second, which is what makes inheritance expressible.
func encodeAuth(auth *domain.Auth) (sql.NullString, error) {
	if auth == nil {
		return sql.NullString{}, nil
	}
	encoded, err := json.Marshal(auth)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("encoding auth: %w", err)
	}
	return sql.NullString{String: string(encoded), Valid: true}, nil
}

// A collection's variables are stored as a blob for the reason auth is: they are read and
// written as a whole set, and nothing queries one of them. NULL is a level that set none.
func decodeVariables(raw sql.NullString, what string) ([]domain.Variable, error) {
	if !raw.Valid {
		return nil, nil
	}
	var value []domain.Variable
	if err := json.Unmarshal([]byte(raw.String), &value); err != nil {
		return nil, fmt.Errorf("reading the variables of %s: %w", what, err)
	}
	return value, nil
}

func encodeVariables(variables []domain.Variable) (sql.NullString, error) {
	if len(variables) == 0 {
		return sql.NullString{}, nil
	}
	encoded, err := json.Marshal(variables)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("encoding variables: %w", err)
	}
	return sql.NullString{String: string(encoded), Valid: true}, nil
}

// SaveNode writes one node whole: a node is edited in one place — the card in "Collections" — and
// saved by one gesture.
func (s *Store) SaveNode(ctx context.Context, node domain.CollectionNode) error {
	params, err := json.Marshal(domain.OrEmpty(node.Params))
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	headers, err := json.Marshal(domain.OrEmpty(node.Headers))
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	cookies, err := json.Marshal(domain.OrEmpty(node.Cookies))
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	auth, err := encodeAuth(node.Auth)
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	scripts, err := encodeScripts(node.Scripts)
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	form, err := json.Marshal(domain.OrEmpty(node.Form))
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}

	now := time.Now().UnixMilli()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO collection_nodes (id, collection_id, name, position, description,
		                               auth_json, scripts_json, method, url, params_json, headers_json,
		                               body, body_kind, form_json, body_file, cookies_json, created_at,
		                               updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, position = excluded.position, description = excluded.description,
		   auth_json = excluded.auth_json, scripts_json = excluded.scripts_json,
		   method = excluded.method, url = excluded.url,
		   params_json = excluded.params_json, headers_json = excluded.headers_json,
		   body = excluded.body, body_kind = excluded.body_kind, form_json = excluded.form_json,
		   body_file = excluded.body_file,
		   cookies_json = excluded.cookies_json, updated_at = excluded.updated_at`,
		node.ID, node.CollectionID, node.Name, node.Position,
		nullIfEmpty(node.Description), auth, scripts, nullIfEmpty(node.Method), nullIfEmpty(node.URL),
		string(params), string(headers), node.Body, string(domain.KindOf(node.BodyKind)), string(form),
		node.BodyFile, string(cookies), now, now)
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	return nil
}

// DeleteCollection removes a collection and, through the cascade, its whole tree.
func (s *Store) DeleteCollection(ctx context.Context, workspaceID, id string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM collections WHERE workspace_id = ? AND id = ?`, workspaceID, id); err != nil {
		return fmt.Errorf("deleting collection %s: %w", id, err)
	}
	return nil
}

// DeleteNode removes a node and, through the cascade, everything under it.
func (s *Store) DeleteNode(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM collection_nodes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("deleting node %s: %w", id, err)
	}
	return nil
}

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
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO collection_run_results (run_id, node_id, position, status, ok, duration_us, error,
		                                     record_id, assertions_passed, assertions_total)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(run_id, position) DO UPDATE SET
		   status = excluded.status, ok = excluded.ok,
		   duration_us = excluded.duration_us, error = excluded.error,
		   record_id = excluded.record_id,
		   assertions_passed = excluded.assertions_passed,
		   assertions_total = excluded.assertions_total`,
		runID, result.NodeID, result.Position, result.Status, result.OK, result.DurationUs, result.Error,
		nullIfEmpty(result.RecordID), result.AssertionsPassed, result.AssertionsTotal)
	if err != nil {
		return fmt.Errorf("saving a result of run %s: %w", runID, err)
	}
	return nil
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
		`SELECT node_id, position, status, ok, duration_us, error, ifnull(record_id, ''),
		        assertions_passed, assertions_total
		   FROM collection_run_results WHERE run_id = ? ORDER BY position`, run.ID)
	if err != nil {
		return domain.CollectionRun{}, false, fmt.Errorf("reading the run %s: %w", run.ID, err)
	}
	defer rows.Close()

	run.Results = []domain.CollectionRunResult{}
	for rows.Next() {
		var (
			result domain.CollectionRunResult
			status sql.NullInt64
		)
		if err := rows.Scan(&result.NodeID, &result.Position, &status, &result.OK,
			&result.DurationUs, &result.Error, &result.RecordID, &result.AssertionsPassed,
			&result.AssertionsTotal); err != nil {
			return domain.CollectionRun{}, false, fmt.Errorf("reading the run %s: %w", run.ID, err)
		}
		if status.Valid {
			code := int(status.Int64)
			result.Status = &code
		}
		run.Results = append(run.Results, result)
	}
	return run, true, rows.Err()
}

// The empty id is the top level. A level's requests and its collections are numbered in one
// sequence, so both tables are asked: a new row lands after what it follows, not at a group's end.
func (s *Store) NextPosition(ctx context.Context, workspaceID, collectionID string) (int64, error) {
	var next int64
	err := s.db.QueryRowContext(ctx,
		`SELECT ifnull(max(position), -1) + 1 FROM (
		   SELECT position FROM collection_nodes WHERE collection_id = ?
		    UNION ALL
		   SELECT position FROM collections
		    WHERE workspace_id = ? AND ifnull(parent_id, '') = ?)`,
		collectionID, workspaceID, collectionID).Scan(&next)
	if err != nil {
		return 0, fmt.Errorf("reading the next position in %s: %w", collectionID, err)
	}
	return next, nil
}
