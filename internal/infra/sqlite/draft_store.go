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

// Draft reads one draft. A database that has never seen it answers ErrNotFound, which is how the
// draft feature knows to start on a fresh one rather than on an empty request.
func (s *Store) Draft(ctx context.Context, id string) (domain.Draft, error) {
	var (
		draft                          domain.Draft
		params, headers, auth, cookies string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, revision, method, url, params_json, headers_json, auth_json, body, cookies_json
		   FROM drafts WHERE id = ?`, id).
		Scan(&draft.ID, &draft.Revision, &draft.Method, &draft.URL, &params, &headers, &auth,
			&draft.Body, &cookies)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Draft{}, fmt.Errorf("draft %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Draft{}, fmt.Errorf("reading draft %s: %w", id, err)
	}

	for _, part := range []struct {
		raw  string
		into any
		what string
	}{
		{params, &draft.Params, "параметры"},
		{headers, &draft.Headers, "заголовки"},
		{auth, &draft.Auth, "авторизация"},
		{cookies, &draft.Cookies, "куки"},
	} {
		if err := json.Unmarshal([]byte(part.raw), part.into); err != nil {
			return domain.Draft{}, fmt.Errorf("reading the %s of draft %s: %w", part.what, id, err)
		}
	}
	return draft, nil
}

// SaveDraft writes a draft whole: it is one row's worth of state, and a partial write of it would
// be a request composed of two different moments.
func (s *Store) SaveDraft(ctx context.Context, draft domain.Draft) error {
	params, err := json.Marshal(orEmptyRows(draft.Params))
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}
	headers, err := json.Marshal(orEmptyRows(draft.Headers))
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}
	auth, err := json.Marshal(draft.Auth)
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}
	cookies, err := json.Marshal(orEmptyCookies(draft.Cookies))
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO drafts (id, revision, method, url, params_json, headers_json, auth_json, body,
		                     cookies_json, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   revision = excluded.revision, method = excluded.method, url = excluded.url,
		   params_json = excluded.params_json, headers_json = excluded.headers_json,
		   auth_json = excluded.auth_json, body = excluded.body,
		   cookies_json = excluded.cookies_json, updated_at = excluded.updated_at`,
		draft.ID, draft.Revision, draft.Method, draft.URL, string(params), string(headers),
		string(auth), draft.Body, string(cookies), time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}
	return nil
}

func orEmptyRows(rows []domain.Row) []domain.Row {
	if rows == nil {
		return []domain.Row{}
	}
	return rows
}
