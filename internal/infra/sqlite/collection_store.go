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

// Collections reads every collection with its tree. The nodes come in one query and are nested here
// rather than in the window: the order they are drawn in is the order this table keeps, and a tree
// assembled in two places is a tree that can disagree with itself.
func (s *Store) Collections(ctx context.Context) ([]domain.Collection, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, position, created_at, updated_at, auth_json
		   FROM collections ORDER BY position, created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing collections: %w", err)
	}
	defer rows.Close()

	out := []domain.Collection{}
	for rows.Next() {
		var (
			c    domain.Collection
			auth sql.NullString
		)
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Position, &c.CreatedAt, &c.UpdatedAt, &auth); err != nil {
			return nil, fmt.Errorf("listing collections: %w", err)
		}
		if auth.Valid {
			var value domain.Auth
			if err := json.Unmarshal([]byte(auth.String), &value); err != nil {
				return nil, fmt.Errorf("reading the auth of collection %s: %w", c.ID, err)
			}
			c.Auth = &value
		}
		c.Items = []domain.CollectionNode{}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing collections: %w", err)
	}
	if len(out) == 0 {
		return out, nil
	}

	nodes, err := s.nodes(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Items = nest(out[i].ID, nodes)
	}
	return out, nil
}

// nest hangs the flat rows on their parents. Children are grouped by parent first, so a node is
// placed by a lookup instead of by walking the tree for its parent — and the order the rows came in
// is the order the tree keeps.
func nest(collectionID string, nodes []domain.CollectionNode) []domain.CollectionNode {
	byParent := map[string][]domain.CollectionNode{}
	for _, node := range nodes {
		if node.CollectionID == collectionID {
			byParent[node.ParentID] = append(byParent[node.ParentID], node)
		}
	}

	var build func(parent string) []domain.CollectionNode
	build = func(parent string) []domain.CollectionNode {
		items := byParent[parent]
		for i := range items {
			items[i].Items = build(items[i].ID)
		}
		return items
	}
	return build("")
}

// Node reads one node with everything a request is made of, which is what opening it needs and what
// the tree deliberately left out.
func (s *Store) Node(ctx context.Context, id string) (domain.CollectionNode, error) {
	var (
		node                        domain.CollectionNode
		parentID                    sql.NullString
		params, headers, cookies    string
		auth                        sql.NullString
		description                 sql.NullString
		url, body, method, bodyKind sql.NullString
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, collection_id, parent_id, kind, name, position, description, auth_json, method, url,
		        params_json, headers_json, body, body_kind, cookies_json, created_at, updated_at
		   FROM collection_nodes WHERE id = ?`, id).
		Scan(&node.ID, &node.CollectionID, &parentID, &node.Kind, &node.Name, &node.Position, &description,
			&auth, &method, &url, &params, &headers, &body, &bodyKind, &cookies,
			&node.CreatedAt, &node.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CollectionNode{}, fmt.Errorf("collection node %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.CollectionNode{}, fmt.Errorf("reading collection node %s: %w", id, err)
	}

	node.ParentID = parentID.String
	node.Description = description.String
	node.Method = method.String
	node.URL = url.String
	node.Body = body.String
	for _, part := range []struct {
		raw  string
		into any
		what string
	}{
		{params, &node.Params, "параметры"},
		{headers, &node.Headers, "заголовки"},
		{cookies, &node.Cookies, "куки"},
	} {
		if err := json.Unmarshal([]byte(part.raw), part.into); err != nil {
			return domain.CollectionNode{}, fmt.Errorf("reading the %s of node %s: %w", part.what, id, err)
		}
	}
	if auth.Valid {
		var value domain.Auth
		if err := json.Unmarshal([]byte(auth.String), &value); err != nil {
			return domain.CollectionNode{}, fmt.Errorf("reading the auth of node %s: %w", id, err)
		}
		node.Auth = &value
	}
	return node, nil
}

// nodes reads every node of every collection, flat, in the order its tree draws them.
func (s *Store) nodes(ctx context.Context) ([]domain.CollectionNode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, collection_id, parent_id, kind, name, position, method
		   FROM collection_nodes ORDER BY collection_id, position, created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing collection nodes: %w", err)
	}
	defer rows.Close()

	out := []domain.CollectionNode{}
	for rows.Next() {
		var (
			node     domain.CollectionNode
			parentID sql.NullString
			method   sql.NullString
		)
		if err := rows.Scan(&node.ID, &node.CollectionID, &parentID, &node.Kind, &node.Name, &node.Position, &method); err != nil {
			return nil, fmt.Errorf("listing collection nodes: %w", err)
		}
		node.ParentID = parentID.String
		node.Method = method.String
		out = append(out, node)
	}
	return out, rows.Err()
}

// SaveCollection writes a collection's own row: its name, its description, its place, and the auth
// everything inside it inherits.
func (s *Store) SaveCollection(ctx context.Context, c domain.Collection) error {
	auth, err := encodeAuth(c.Auth)
	if err != nil {
		return fmt.Errorf("saving collection %s: %w", c.ID, err)
	}

	now := time.Now().UnixMilli()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO collections (id, name, description, position, auth_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, description = excluded.description,
		   position = excluded.position, auth_json = excluded.auth_json,
		   updated_at = excluded.updated_at`,
		c.ID, c.Name, c.Description, c.Position, auth, now, now); err != nil {
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

// SaveNode writes one node whole. Everything a request is made of is here because everything about a
// node is edited in one place — the card in "Коллекциях" — and saved by one gesture.
func (s *Store) SaveNode(ctx context.Context, node domain.CollectionNode) error {
	params, err := json.Marshal(orEmptyRows(node.Params))
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	headers, err := json.Marshal(orEmptyRows(node.Headers))
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	cookies, err := json.Marshal(orEmptyCookies(node.Cookies))
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	auth, err := encodeAuth(node.Auth)
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}

	now := time.Now().UnixMilli()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO collection_nodes (id, collection_id, parent_id, kind, name, position, description,
		                               auth_json, method, url, params_json, headers_json, body, body_kind,
		                               cookies_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'raw', ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, position = excluded.position, description = excluded.description,
		   auth_json = excluded.auth_json, method = excluded.method, url = excluded.url,
		   params_json = excluded.params_json, headers_json = excluded.headers_json,
		   body = excluded.body, cookies_json = excluded.cookies_json, updated_at = excluded.updated_at`,
		node.ID, node.CollectionID, nullIfEmpty(node.ParentID), string(node.Kind), node.Name, node.Position,
		nullIfEmpty(node.Description), auth, nullIfEmpty(node.Method), nullIfEmpty(node.URL),
		string(params), string(headers), node.Body, string(cookies), now, now)
	if err != nil {
		return fmt.Errorf("saving node %s: %w", node.ID, err)
	}
	return nil
}

// DeleteCollection removes a collection and, through the cascade, its whole tree.
func (s *Store) DeleteCollection(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM collections WHERE id = ?`, id); err != nil {
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

// NextPosition is one past the last sibling, which is where a new node lands: the end of its group,
// the way a new row lands in a list.
func (s *Store) NextPosition(ctx context.Context, collectionID string, parentID string) (int64, error) {
	var next int64
	err := s.db.QueryRowContext(ctx,
		`SELECT ifnull(max(position), -1) + 1 FROM collection_nodes
		  WHERE collection_id = ? AND ifnull(parent_id, '') = ?`,
		collectionID, parentID).Scan(&next)
	if err != nil {
		return 0, fmt.Errorf("reading the next position in %s: %w", collectionID, err)
	}
	return next, nil
}
