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

// The answers in the Auth chip are texts like any other: a variable in one of them is filled in on
// the way out and left as a mask in the copy that is written down, and what the scheme makes of them
// is what the request goes out with.
func TestTheAuthFieldsReachTheSchemeResolved(t *testing.T) {
	ctx := context.Background()
	uc, _, auth := newUseCaseWithAuth()
	if err := uc.Load(ctx); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method: "GET",
		URL:    "https://api.example.com/a",
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	auth.answer = domain.AuthOutput{Headers: []domain.HeaderPair{{Name: "Authorization", Value: "given"}}}
	authWith(t, uc, domain.NewAuth(domain.AuthBearer).With("token", "{{token}}"))

	prepared, err := uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if got := auth.asked[0].Get("token"); got != "abc123" {
		t.Errorf("the scheme was asked with %q, want the resolved variable", got)
	}
	// The record outlives the send, so the secret it carries is the mask the environment gave.
	if got := auth.asked[1].Get("token"); got != "••••" {
		t.Errorf("the second pass asked with %q, want the mask rather than the value", got)
	}
	if value, ok := headerOf(prepared.Headers, "Authorization"); !ok || value != "given" {
		t.Errorf("headers = %+v, want what the scheme answered in them", prepared.Headers)
	}
	if value, _ := headerOf(prepared.MaskedHeaders, "Authorization"); value != "given" {
		t.Errorf("masked headers = %+v, want the masked answer in them", prepared.MaskedHeaders)
	}
}

// A header the user wrote by hand is the more precise answer, and two Authorization headers are a
// request no server reads the way the window drew it.
func TestAWrittenAuthorizationHeaderWins(t *testing.T) {
	ctx := context.Background()
	uc, _, auth := newUseCaseWithAuth()
	if err := uc.Load(ctx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	auth.answer = domain.AuthOutput{Headers: []domain.HeaderPair{{Name: "Authorization", Value: "from-the-chip"}}}

	if _, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method:  "GET",
		URL:     "https://api.example.com/a",
		Headers: []domain.HeaderPair{{Name: "authorization", Value: "Token hand-written"}},
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	authWith(t, uc, domain.NewAuth(domain.AuthBearer).With("token", "abc123"))

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

// A scheme told to travel in the query string puts its parameter on the address being sent — and not
// among the draft's rows, which are what the window edits and what the next keystroke in the address
// bar would rewrite out of it.
func TestAQueryParameterGoesOnTheAddressAndNotInTheRows(t *testing.T) {
	ctx := context.Background()
	uc, _, auth := newUseCaseWithAuth()
	if err := uc.Load(ctx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	auth.answer = domain.AuthOutput{Query: []domain.HeaderPair{{Name: "X-API-Key", Value: "s3cret"}}}

	if _, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method: "GET",
		URL:    "https://api.example.com/a?include=author",
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	authWith(t, uc, domain.NewAuth(domain.AuthAPIKey).With("key", "X-API-Key"))

	prepared, err := uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	want := "https://api.example.com/a?include=author&X-API-Key=s3cret"
	if prepared.URL != want {
		t.Errorf("url = %q, want %q", prepared.URL, want)
	}

	state, err := uc.Snapshot(ctx, domain.DraftCommandLine)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if state.Draft.URL != "https://api.example.com/a?include=author" {
		t.Errorf("the draft's address = %q, want it untouched by the projection", state.Draft.URL)
	}
}

// A parameter the user already wrote under that name is the more precise answer, for the reason a
// written header is.
func TestAWrittenParameterWins(t *testing.T) {
	ctx := context.Background()
	uc, _, auth := newUseCaseWithAuth()
	if err := uc.Load(ctx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	auth.answer = domain.AuthOutput{Query: []domain.HeaderPair{{Name: "X-API-Key", Value: "from-the-chip"}}}

	if _, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method: "GET",
		URL:    "https://api.example.com/a?X-API-Key=hand-written",
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	authWith(t, uc, domain.NewAuth(domain.AuthAPIKey).With("key", "X-API-Key"))

	prepared, err := uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if prepared.URL != "https://api.example.com/a?X-API-Key=hand-written" {
		t.Errorf("url = %q, want the parameter the user wrote and only that one", prepared.URL)
	}
}

// A credential that cannot be put on the request yet travels to the engine beside it rather than on
// it: nothing above the engine has a header to show for a Digest challenge that has not arrived.
func TestACredentialTheRequestCannotCarryGoesToTheEngine(t *testing.T) {
	ctx := context.Background()
	line := newLine(t)
	line.auth.answer = domain.AuthOutput{
		Digest: &domain.DigestCredentials{Username: "user", Password: "pass"},
	}

	prepared, err := line.uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if prepared.Digest == nil || prepared.Digest.Username != "user" {
		t.Fatalf("digest = %+v, want the credential handed on", prepared.Digest)
	}
	if _, ok := headerOf(prepared.Headers, "Authorization"); ok {
		t.Errorf("headers = %+v, want no authorization: there is nothing to put there", prepared.Headers)
	}
}

// A row the user wrote under that name is the more precise answer, and a challenge answered over it
// would be an answer to a request nobody asked about.
func TestAWrittenAuthorizationHeaderTakesTheChallengeAway(t *testing.T) {
	ctx := context.Background()
	line := newLine(t)
	line.auth.answer = domain.AuthOutput{
		Digest: &domain.DigestCredentials{Username: "user", Password: "pass"},
	}
	if _, err := line.uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method:  "GET",
		URL:     "https://api.example.com/a",
		Headers: []domain.HeaderPair{{Name: "Authorization", Value: "Token hand-written"}},
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	prepared, err := line.uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if prepared.Digest != nil {
		t.Errorf("digest = %+v, want none while a written header stands", prepared.Digest)
	}
}

// «Наследовать» is answered by whoever knows the tree: the draft is told what the levels above it
// said, and a request with nothing above it sends no credentials at all.
func TestInheritedAuthIsWhatTheCallerResolved(t *testing.T) {
	ctx := context.Background()
	uc, _, auth := newUseCaseWithAuth()
	if err := uc.Load(ctx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	auth.answer = domain.AuthOutput{Headers: []domain.HeaderPair{{Name: "Authorization", Value: "from-the-collection"}}}

	if _, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method: "GET",
		URL:    "https://api.example.com/a",
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	authWith(t, uc, domain.NewAuth(domain.AuthInherit))

	inherited := domain.NewAuth(domain.AuthBearer).With("token", "from-the-collection")
	prepared, err := uc.Prepared(ctx, domain.DraftCommandLine, &inherited)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if got := auth.asked[0].Get("token"); got != "from-the-collection" {
		t.Errorf("the scheme was asked with %q, want what the level above answered", got)
	}
	if value, ok := headerOf(prepared.Headers, "Authorization"); !ok || value != "from-the-collection" {
		t.Errorf("headers = %+v, want the inherited authorization in them", prepared.Headers)
	}

	// Nothing above is not «нет» written down somewhere: the request simply goes out with nothing.
	auth.answer = domain.AuthOutput{}
	prepared, err = uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}
	if !auth.last().IsNone() {
		t.Errorf("auth = %q, want «нет» when there is nothing above to inherit", auth.last().Type)
	}
	if _, ok := headerOf(prepared.Headers, "Authorization"); ok {
		t.Errorf("headers = %+v, want no authorization when there is nothing above to inherit", prepared.Headers)
	}
}
