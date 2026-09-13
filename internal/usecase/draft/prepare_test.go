package draft

import (
	"context"
	"testing"

	"json-inspector/internal/domain"
)

// authWith sets the command line's auth, which is what the chip does when it is clicked.
func authWith(t *testing.T, uc *UseCase, auth domain.Auth) {
	t.Helper()
	if _, err := uc.SetAuth(context.Background(), domain.DraftCommandLine, auth); err != nil {
		t.Fatalf("SetAuth: %v", err)
	}
}

func headerOf(headers []domain.HeaderPair, name string) (string, bool) {
	for _, header := range headers {
		if header.Name == name {
			return header.Value, true
		}
	}
	return "", false
}

// The choice in the Auth chip is where the Authorization header comes from, and the token in it is a
// text like any other: a variable in it is filled in on the way out and left as a mask in the copy
// that is written down.
func TestTheAuthBecomesAnAuthorizationHeader(t *testing.T) {
	ctx := context.Background()
	uc, _ := loaded(t)

	if _, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method: "GET",
		URL:    "https://api.example.com/a",
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	authWith(t, uc, domain.Auth{Type: domain.AuthBearer, Token: "{{token}}"})
	prepared, err := uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if value, ok := headerOf(prepared.Headers, "Authorization"); !ok || value != "Bearer abc123" {
		t.Errorf("headers = %+v, want the resolved token in them", prepared.Headers)
	}
	// The record outlives the send, so the secret it carries is the mask the environment gave.
	if value, _ := headerOf(prepared.MaskedHeaders, "Authorization"); value != "Bearer ••••" {
		t.Errorf("masked headers = %+v, want the mask rather than the value", prepared.MaskedHeaders)
	}

	authWith(t, uc, domain.Auth{Type: domain.AuthBasic, Token: "user:pass"})
	prepared, err = uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if value, _ := headerOf(prepared.Headers, "Authorization"); value != "Basic dXNlcjpwYXNz" {
		t.Errorf("basic authorization = %q, want the credential encoded", value)
	}

	authWith(t, uc, domain.Auth{Type: domain.AuthNone})
	prepared, err = uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if _, ok := headerOf(prepared.Headers, "Authorization"); ok {
		t.Errorf("headers = %+v, want none for a request with no authorization", prepared.Headers)
	}
}

// A header the user wrote by hand is the more precise answer, and two Authorization headers are a
// request no server reads the way the window drew it.
func TestAWrittenAuthorizationHeaderWins(t *testing.T) {
	ctx := context.Background()
	uc, _ := loaded(t)

	if _, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method:  "GET",
		URL:     "https://api.example.com/a",
		Headers: []domain.HeaderPair{{Name: "authorization", Value: "Token hand-written"}},
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	authWith(t, uc, domain.Auth{Type: domain.AuthBearer, Token: "from-the-chip"})

	prepared, err := uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if len(prepared.Headers) != 1 {
		t.Fatalf("headers = %+v, want the one row and nothing beside it", prepared.Headers)
	}
	if prepared.Headers[0].Value != "Token hand-written" {
		t.Errorf("authorization = %q, want what was written", prepared.Headers[0].Value)
	}
}

// «Наследовать» is answered by whoever knows the tree: the draft is told what the levels above it
// said, and a request with nothing above it sends no credentials at all.
func TestInheritedAuthIsWhatTheCallerResolved(t *testing.T) {
	ctx := context.Background()
	uc, _ := loaded(t)

	if _, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method: "GET",
		URL:    "https://api.example.com/a",
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	authWith(t, uc, domain.Auth{Type: domain.AuthInherit})

	inherited := domain.Auth{Type: domain.AuthBearer, Token: "from-the-collection"}
	prepared, err := uc.Prepared(ctx, domain.DraftCommandLine, &inherited)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if value, ok := headerOf(prepared.Headers, "Authorization"); !ok || value != "Bearer from-the-collection" {
		t.Errorf("headers = %+v, want the inherited authorization in them", prepared.Headers)
	}

	prepared, err = uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if _, ok := headerOf(prepared.Headers, "Authorization"); ok {
		t.Errorf("headers = %+v, want none when there is nothing above to inherit", prepared.Headers)
	}
}
