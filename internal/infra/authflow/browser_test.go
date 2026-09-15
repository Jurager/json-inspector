package authflow

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"json-inspector/internal/domain"
)

// browserThatFollows is a browser as far as this package can tell: it is handed the provider's page
// and does what a person would do there — say yes and be sent back. What it is sent back with is
// the test's to choose, which is the whole point: the provider is the one party a test cannot be.
type browserThatFollows struct {
	t      *testing.T
	answer func(authorize url.URL) (redirect string, fragment string)
	opened string
}

func (b *browserThatFollows) OpenURL(raw string) error {
	b.opened = raw
	authorize, err := url.Parse(raw)
	if err != nil {
		b.t.Fatalf("the page to open is not an address: %v", err)
	}
	redirect, fragment := b.answer(*authorize)
	b.follow(redirect, fragment)
	return nil
}

// follow is the trip back: the browser asks for the address the provider would have sent it to, and
// if the answer is one a fragment carries, it does what the page there tells it to.
func (b *browserThatFollows) follow(redirect, fragment string) {
	if redirect == "" {
		// A browser that opens nothing, which is what a test of the waiting needs.
		return
	}
	if fragment == "" {
		if _, err := http.Get(redirect); err != nil {
			b.t.Errorf("following the redirect: %v", err)
		}
		return
	}
	// The fragment never reaches a server, so the page serves a script that posts it back. A browser
	// without a script engine is one that does the script's work itself.
	page, err := http.Get(redirect)
	if err != nil {
		b.t.Errorf("loading the page: %v", err)
		return
	}
	defer page.Body.Close()

	target := strings.TrimSuffix(redirect, "/callback") + "/token"
	if _, err := http.PostForm(target, formOf(fragment)); err != nil {
		b.t.Errorf("posting the answer back: %v", err)
	}
}

func formOf(fragment string) url.Values {
	values, err := url.ParseQuery(fragment)
	if err != nil {
		return url.Values{}
	}
	return values
}

// The code grant is the whole trip: the person is sent to the provider, comes back with a code, and
// the app exchanges it — with a PKCE verifier, so that a code read off the redirect is useless to
// anybody else.
func TestTheCodeGrantSignsInAndExchanges(t *testing.T) {
	endpoint := &spyTokenEndpoint{token: "issued-for-a-code", expiry: 3600}
	srv := endpoint.server(t)

	browser := &browserThatFollows{t: t, answer: func(authorize url.URL) (string, string) {
		query := authorize.Query()
		if query.Get("response_type") != "code" {
			t.Errorf("response_type = %q, want the code grant", query.Get("response_type"))
		}
		if query.Get("code_challenge") == "" {
			t.Error("no code challenge: a code read off the redirect would be anyone's to exchange")
		}
		return query.Get("redirect_uri") + "?code=the-code&state=" + query.Get("state"), ""
	}}

	auth := oauthAuth(map[string]string{
		"grant": "authorization_code", "authUrl": "https://idp.example.com/authorize",
		"tokenUrl": srv.URL, "clientId": "id", "clientSecret": "secret",
	})
	// The sign-in talks to the provider with the plain client: it is a browser address, not the app's
	// own engine, and the token endpoint is the test's.
	client := srv.Client()
	materializer := New(client, browser)

	out, err := materializer.Materialize(context.Background(), auth, domain.AuthRequest{})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if value, _ := headerOf(out, "Authorization"); value != "Bearer issued-for-a-code" {
		t.Errorf("authorization = %q, want the token the code was exchanged for", value)
	}

	form := endpoint.got[0]
	if form["grant_type"] != "authorization_code" || form["code"] != "the-code" {
		t.Errorf("form = %+v, want the code exchanged for a token", form)
	}
	if form["code_verifier"] == "" {
		t.Error("the exchange carried no verifier: the challenge was for nothing")
	}
	// The address the provider is told to come back to is this machine, and the same one the app waits
	// on: two different ports would be a sign-in that never lands.
	if !strings.HasPrefix(browser.opened, "https://idp.example.com/authorize?") {
		t.Errorf("opened %q, want the provider's authorization endpoint", browser.opened)
	}
}

// The implicit grant is the same trip without the exchange: the provider hands the token over
// directly, in the part of a URL no server ever sees.
func TestTheImplicitGrantTakesTheTokenFromTheFragment(t *testing.T) {
	browser := &browserThatFollows{t: t, answer: func(authorize url.URL) (string, string) {
		query := authorize.Query()
		if query.Get("response_type") != "token" {
			t.Errorf("response_type = %q, want the grant that gets the token itself",
				query.Get("response_type"))
		}
		return query.Get("redirect_uri"),
			"access_token=from-the-fragment&expires_in=60&state=" + query.Get("state")
	}}

	auth := oauthAuth(map[string]string{
		"grant": "implicit", "authUrl": "https://idp.example.com/authorize", "clientId": "id",
	})
	out, err := New(nil, browser).Materialize(context.Background(), auth, domain.AuthRequest{})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if value, _ := headerOf(out, "Authorization"); value != "Bearer from-the-fragment" {
		t.Errorf("authorization = %q, want the token the provider handed over", value)
	}
}

// An answer naming another sign-in is one that arrived from somewhere else, and its code is not
// exchanged. The sign-in itself goes on waiting: a request that landed on the port by accident is
// not the same thing as the answer that is still coming, and giving up over one would end a
// legitimate sign-in because something port-scanned the machine.
func TestAnAnswerFromAnotherSignInIsNotTaken(t *testing.T) {
	browser := &browserThatFollows{t: t, answer: func(authorize url.URL) (string, string) {
		return authorize.Query().Get("redirect_uri") + "?code=theirs&state=not-our-state", ""
	}}
	auth := oauthAuth(map[string]string{
		"grant": "authorization_code", "authUrl": "https://idp.example.com/authorize",
		"tokenUrl": "https://idp.example.com/token", "clientId": "id",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := New(nil, browser).Materialize(ctx, auth, domain.AuthRequest{})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("an answer from another sign-in was accepted")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the sign-in neither took the answer nor gave up")
	}
}

// A person who says no is not a failure of the app, and the provider's own word for it travels
// back.
func TestARefusalAtTheProviderTravelsBack(t *testing.T) {
	browser := &browserThatFollows{t: t, answer: func(authorize url.URL) (string, string) {
		return authorize.Query().Get("redirect_uri") + "?error=access_denied&state=" +
			authorize.Query().Get("state"), ""
	}}
	auth := oauthAuth(map[string]string{
		"grant": "authorization_code", "authUrl": "https://idp.example.com/authorize",
		"tokenUrl": "https://idp.example.com/token", "clientId": "id",
	})

	_, err := New(nil, browser).Materialize(context.Background(), auth, domain.AuthRequest{})
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("err = %v, want the provider's own word for what happened", err)
	}
}

// The wait ends with the context: a sign-in nobody came back to must not hold a send open.
func TestTheWaitEndsWithTheContext(t *testing.T) {
	// A browser that opens nothing, so nothing ever comes back.
	browser := &browserThatFollows{t: t, answer: func(url.URL) (string, string) { return "", "" }}
	auth := oauthAuth(map[string]string{
		"grant": "authorization_code", "authUrl": "https://idp.example.com/authorize",
		"tokenUrl": "https://idp.example.com/token", "clientId": "id",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := New(nil, browser).Materialize(ctx, auth, domain.AuthRequest{})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("a sign-in that was given up on reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("giving up on a sign-in did not end the wait")
	}
}
