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
func (s *Store) Draft(
	ctx context.Context,
	workspaceID string,
	id domain.DraftID,
) (domain.Draft, error) {
	var (
		draft                                domain.Draft
		params, headers, auth, cookies, form string
		bodyKind                             string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, revision, method, url, params_json, headers_json, auth_json, body, body_kind,
		        form_json, body_file, cookies_json, environment_id
		   FROM drafts WHERE workspace_id = ? AND id = ?`, workspaceID, id).
		Scan(&draft.ID, &draft.Revision, &draft.Method, &draft.URL, &params, &headers, &auth,
			&draft.Body, &bodyKind, &form, &draft.BodyFile, &cookies, &draft.EnvironmentID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Draft{}, fmt.Errorf("draft %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Draft{}, fmt.Errorf("reading draft %s: %w", id, err)
	}
	// The column carries a default, so this is only the wire's own gap: a kind that arrived empty
	// means text, and text is what raw is.
	draft.BodyKind = domain.KindOf(domain.BodyKind(bodyKind))

	for _, part := range []struct {
		raw  string
		into any
		what string
	}{
		{params, &draft.Params, "params"},
		{headers, &draft.Headers, "headers"},
		{auth, &draft.Auth, "auth"},
		{cookies, &draft.Cookies, "cookies"},
		{form, &draft.Form, "form fields"},
	} {
		if err := json.Unmarshal([]byte(part.raw), part.into); err != nil {
			return domain.Draft{}, fmt.Errorf("reading the %s of draft %s: %w", part.what, id, err)
		}
	}
	return draft, nil
}

// SaveDraft writes a draft whole: it is one row's worth of state, and a partial write of it would
// be a request composed of two different moments.
func (s *Store) SaveDraft(ctx context.Context, workspaceID string, draft domain.Draft) error {
	params, err := json.Marshal(domain.OrEmpty(draft.Params))
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}
	headers, err := json.Marshal(domain.OrEmpty(draft.Headers))
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}
	auth, err := json.Marshal(draft.Auth)
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}
	cookies, err := json.Marshal(domain.OrEmpty(draft.Cookies))
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}
	form, err := json.Marshal(domain.OrEmpty(draft.Form))
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO drafts (workspace_id, id, revision, method, url, params_json, headers_json,
		                     auth_json, body, body_kind, form_json, body_file, cookies_json,
		                     environment_id, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(workspace_id, id) DO UPDATE SET
		   revision = excluded.revision, method = excluded.method, url = excluded.url,
		   params_json = excluded.params_json, headers_json = excluded.headers_json,
		   auth_json = excluded.auth_json, body = excluded.body,
		   body_kind = excluded.body_kind, form_json = excluded.form_json,
		   body_file = excluded.body_file,
		   cookies_json = excluded.cookies_json, environment_id = excluded.environment_id,
		   updated_at = excluded.updated_at`,
		workspaceID, draft.ID, draft.Revision, draft.Method, draft.URL, string(params),
		string(headers), string(auth), draft.Body, string(domain.KindOf(draft.BodyKind)),
		string(form), draft.BodyFile, string(cookies), draft.EnvironmentID, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("saving draft %s: %w", draft.ID, err)
	}
	return nil
}
