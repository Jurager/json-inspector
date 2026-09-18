package accountclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"json-inspector/internal/domain"
)

// The device flow spends its whole wait being told "not yet": the standard answers a device that
// asks too early with a 400 and `authorization_pending` in the body. A client that reads the status
// as the verdict ends every sign-in at its first poll — before anybody has had the chance to
// confirm the code — and this is the test that keeps that from coming back.
func TestAnAnswerThatSaysTheSignInIsStillGoingIsNotARefusal(t *testing.T) {
	endpoint := tokenEndpoint(t, http.StatusBadRequest, `{"error":"authorization_pending"}`)

	_, waiting, err := New(endpoint.Client()).token(context.Background(), endpoint.URL, deviceForm())
	if err != nil {
		t.Fatalf("a pending answer was read as a failure: %v", err)
	}
	if !waiting {
		t.Error("the client did not see that it has to ask again")
	}
}

// The other half of the same reading: a server that says no is a no, and the sign-in stops.
func TestARefusalFromTheTokenEndpointEndsTheSignIn(t *testing.T) {
	endpoint := tokenEndpoint(t, http.StatusBadRequest, `{"error":"access_denied"}`)

	_, waiting, err := New(endpoint.Client()).token(context.Background(), endpoint.URL, deviceForm())
	if waiting {
		t.Fatal("a refusal was read as an invitation to keep asking")
	}
	if code := domain.CodeOf(err); code != domain.CodeSignInFailed {
		t.Errorf("the refusal came through as %q, want %q", code, domain.CodeSignInFailed)
	}
}

// And the answer the wait is for: a pair of tokens, with the life the server gave them.
func TestTheTokensAreReadFromAnAnswer(t *testing.T) {
	endpoint := tokenEndpoint(t, http.StatusOK,
		`{"access_token":"access","refresh_token":"refresh","expires_in":3600}`)

	tokens, waiting, err := New(endpoint.Client()).token(context.Background(), endpoint.URL,
		deviceForm())
	if err != nil || waiting {
		t.Fatalf("a good answer was read as waiting=%v, err=%v", waiting, err)
	}
	if tokens.Access != "access" || tokens.Refresh != "refresh" {
		t.Errorf("the tokens came through as %+v", tokens)
	}
	if left := time.Until(tokens.ExpiresAt); left < 59*time.Minute {
		t.Errorf("the access token was given %v to live, want about an hour", left)
	}
}

// tokenEndpoint is the token endpoint saying one thing, and nothing else.
func tokenEndpoint(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func deviceForm() url.Values {
	return url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {"a device code"},
		"client_id":   {clientID},
	}
}
