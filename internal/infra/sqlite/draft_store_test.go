package sqlite

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
)

func sampleDraft() domain.Draft {
	return domain.Draft{
		ID:       domain.DraftCommandLine,
		Revision: 3,
		Method:   "POST",
		URL:      "https://api.example.com/a?x=1&y={{token}}",
		Params: []domain.Row{
			{ID: "p1", Name: "x", Value: "1", Enabled: true},
			{ID: "p2", Name: "y", Value: "{{token}}", Enabled: false},
		},
		Headers: []domain.Row{{ID: "h1", Name: "Authorization", Value: "Bearer {{token}}", Enabled: true}},
		Auth:    domain.Auth{Type: domain.AuthBearer, Token: "{{token}}"},
		Body:    `{"a": 1}`,
		Cookies: []domain.CookieRow{{ID: "c1", Name: "session", Value: "abc", Path: "/", HTTPOnly: true}},
	}
}

func TestDraftRoundTrip(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if _, err := store.Draft(ctx, domain.DraftCommandLine); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a draft that was never saved = %v, want ErrNotFound", err)
	}

	saved := sampleDraft()
	if err := store.SaveDraft(ctx, saved); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	got, err := store.Draft(ctx, domain.DraftCommandLine)
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if got.Method != saved.Method || got.URL != saved.URL || got.Body != saved.Body || got.Revision != 3 {
		t.Errorf("draft = %+v, want the saved one", got)
	}
	if len(got.Params) != 2 || got.Params[1].ID != "p2" || got.Params[1].Enabled {
		t.Errorf("params = %+v, want both rows with what they carried", got.Params)
	}
	if len(got.Headers) != 1 || got.Headers[0].Value != "Bearer {{token}}" {
		t.Errorf("headers = %+v, want the token as it was typed", got.Headers)
	}
	if got.Auth.Type != domain.AuthBearer {
		t.Errorf("auth = %+v", got.Auth)
	}
	if len(got.Cookies) != 1 || got.Cookies[0].HTTPOnly != true || got.Cookies[0].ID != "c1" {
		t.Errorf("cookies = %+v, want the row with its attributes", got.Cookies)
	}
}

// The draft is one row: saving twice replaces what the window was composing before.
func TestSaveDraftReplacesTheOneBefore(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveDraft(ctx, sampleDraft()); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	next := sampleDraft()
	next.Revision = 4
	next.Method = "DELETE"
	next.Params = nil
	if err := store.SaveDraft(ctx, next); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	got, err := store.Draft(ctx, domain.DraftCommandLine)
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if got.Method != "DELETE" || got.Revision != 4 {
		t.Errorf("draft = %+v, want the second save", got)
	}
	if len(got.Params) != 0 {
		t.Errorf("params = %+v, want the empty list the second save had", got.Params)
	}

	var rows int
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM drafts`).Scan(&rows); err != nil {
		t.Fatalf("counting drafts: %v", err)
	}
	if rows != 1 {
		t.Errorf("drafts = %d, want one", rows)
	}
}
