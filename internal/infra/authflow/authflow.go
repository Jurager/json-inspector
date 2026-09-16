// Package authflow turns what a level answered about its authorization into what actually goes on
// the wire. Every scheme is one entry in a table, and nothing here asks what type it is holding —
// the type is the key it was looked up by.
package authflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"json-inspector/internal/domain"
)

// Materializer holds the credentials the schemes in the table hand out. A token is kept in memory
// and never written down: one that is not stored cannot be exported, logged or screenshotted.
type Materializer struct {
	// client is what the app sends every request with, so the token endpoint gets the same proxy and
	// the same timeout as anything else.
	client *http.Client
	// browser is where the two grants that need a person are sent. Nil where there is nothing to open.
	browser Browser
	// exchange is browser as the scheme table asks for it, and nil where browser is nil: the two
	// grants that need one then refuse instead of hanging.
	exchange codeExchange

	mu     sync.Mutex
	tokens map[string]issued
	// pages is what a sign-in puts in the browser. The window hands it over as it mounts — see
	// SetPages — and it is read when a sign-in starts rather than captured, so a language changed
	// while one is waiting is not frozen into the page that ends it.
	pages domain.SignInPages
}

// issued is a token with its expiry. A provider that named none has said all it will, and the token
// is kept until the window closes.
type issued struct {
	token   string
	expires time.Time
}

// expiryDelta is how long before a token dies it is treated as dead. A token good for another
// half-second is not one to send a request with, and asking for another is cheaper than a refusal.
const expiryDelta = 10 * time.Second

// New builds the materializer. A nil browser leaves the two grants that need one unable to run —
// the honest answer where there is nothing to open.
func New(client *http.Client, browser Browser) *Materializer {
	m := &Materializer{client: client, browser: browser, tokens: map[string]issued{}}
	if browser != nil {
		m.exchange = m.signIn
	}
	return m
}

// SetPages is the window handing over the words a sign-in puts in the browser. They cannot be
// written here: the catalogue is the window's, and the language it resolves "system" to is the
// webview's to answer. The native menu's labels arrive the same way and for the same reason.
//
// A window that hands over nothing is not a window asking for a blank page, and it is a real
// caller: the bindings are called by number, so a window running the bundle from before this method
// had its second argument sends one and leaves the rest empty. What stands then is the fallback,
// which is a page in the wrong language — better than a page with no words on it at all.
func (m *Materializer) SetPages(pages domain.SignInPages) {
	if pages == (domain.SignInPages{}) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pages = pages
}

// signIn is the exchange the two browser grants go through. It is a method and not a value built at
// New time because the words it writes arrive later — the window hands them over as it mounts — and
// the page that ends a sign-in has to be in the language that is in force when it starts.
func (m *Materializer) signIn(ctx context.Context, auth domain.Auth) (string, time.Time, error) {
	m.mu.Lock()
	pages := m.pages
	m.mu.Unlock()
	return signIn(ctx, m.browser, auth, pages)
}

// Materialize answers with what the scheme puts on the request. A scheme that is not in the table —
// «None», «Inherit», or one whose working is not written yet — puts nothing there, which is the
// same answer an unfilled field gets.
func (m *Materializer) Materialize(
	ctx context.Context,
	auth domain.Auth,
	req domain.AuthRequest,
) (domain.AuthOutput, error) {
	entry, ok := schemes[auth.Type]
	if !ok {
		return domain.AuthOutput{}, nil
	}
	if entry.fetch == nil {
		return entry.put(auth, req)
	}
	token, err := m.obtain(ctx, auth, entry)
	if err != nil {
		return domain.AuthOutput{}, err
	}
	if token == "" {
		return domain.AuthOutput{}, nil
	}
	return carry(auth, token), nil
}

// Project answers without asking anybody for anything: a window drawing rows while a person types
// is not the moment to go and get a token. A fetched scheme draws nothing until one is held, which
// is what «No token» says.
func (m *Materializer) Project(
	auth domain.Auth,
	req domain.AuthRequest,
) (domain.AuthOutput, error) {
	entry, ok := schemes[auth.Type]
	if !ok {
		return domain.AuthOutput{}, nil
	}
	if entry.fetch == nil {
		return entry.put(auth, req)
	}
	if token, held := m.held(auth); held {
		return carry(auth, token), nil
	}
	return domain.AuthOutput{}, nil
}

// Obtain asks for the token a scheme carries. The window asked for this in so many words, so what
// went wrong is reported rather than swallowed: it is the answer the user is waiting for.
func (m *Materializer) Obtain(ctx context.Context, auth domain.Auth) error {
	entry, ok := schemes[auth.Type]
	if !ok || entry.fetch == nil {
		return nil
	}
	m.Forget(auth)
	_, err := m.obtain(ctx, auth, entry)
	return err
}

// Forget drops a token, so the next send asks for another one; a scheme that carries what it was
// given has nothing to drop.
func (m *Materializer) Forget(auth domain.Auth) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, tokenKey(auth))
}

// Held is whether a scheme has a token at this moment and until when, which is all the window is
// told: it draws «No token» or that there is one, and never the value itself.
func (m *Materializer) Held(auth domain.Auth) domain.AuthToken {
	m.mu.Lock()
	defer m.mu.Unlock()
	found, ok := m.tokens[tokenKey(auth)]
	if !ok {
		return domain.AuthToken{}
	}
	return domain.AuthToken{Held: true, ExpiresAt: found.expires.UnixMilli()}
}

func (m *Materializer) obtain(ctx context.Context, auth domain.Auth, entry scheme) (string, error) {
	if token, held := m.held(auth); held {
		return token, nil
	}
	// The library reads its HTTP client out of the context, so no call site has to thread one through
	// everything that touches a token.
	value, expires, err := entry.fetch(context.WithValue(ctx, oauth2.HTTPClient, m.client), auth,
		m.exchange)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[tokenKey(auth)] = issued{token: value, expires: expires}
	return value, nil
}

func (m *Materializer) held(auth domain.Auth) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found, ok := m.tokens[tokenKey(auth)]
	if !ok || found.token == "" {
		return "", false
	}
	if !found.expires.IsZero() && time.Now().Add(expiryDelta).After(found.expires) {
		return "", false
	}
	return found.token, true
}

// tokenKey names a token by the answers that would fetch it, so two requests that share an
// authorization share the token. Marshalling makes it stable: the same answers are the same string.
func tokenKey(auth domain.Auth) string {
	encoded, err := json.Marshal(auth)
	if err != nil {
		return string(auth.Type)
	}
	return string(encoded)
}

// Absorb turns an edit to a projected row back into an answer to the scheme's fields. A scheme
// whose rows are not editable is never asked — the window knows from AuthOutput.Editable — and the
// error is for the case where it is asked anyway.
func (m *Materializer) Absorb(
	auth domain.Auth,
	target domain.RowKind,
	name, value string,
) (domain.Auth, error) {
	entry, ok := schemes[auth.Type]
	if !ok || entry.absorb == nil {
		return auth, fmt.Errorf("editing the row %s projects: %w", auth.Type, domain.ErrNotAllowed)
	}
	return entry.absorb(auth, target, name, value)
}

// scheme is how a request is authorized: what it puts on the request, and, where it can, how an
// edit to that comes back. The two live together because a scheme taking an edit has to know what
// it wrote.
type scheme struct {
	put putter
	// fetch is a scheme whose credential is asked for. A scheme with one is drawn by the window with a
	// button rather than only fields, and is the only kind with anything to remember between calls.
	fetch fetcher
	// absorb is absent for a scheme whose rows the user may not edit. A credential encoded as a whole
	// is the case: `Basic dXNlcjpwYXNz` does not come apart into a login and a password, and a
	// signature is not a value a person has to begin with.
	absorb absorber
}

// putter is what a scheme writes onto a request, and whether the window may offer to edit it.
type putter func(auth domain.Auth, req domain.AuthRequest) (domain.AuthOutput, error)

// absorber turns an edit to one of those rows back into an answer to the scheme's fields.
type absorber func(auth domain.Auth, target domain.RowKind, name, value string) (domain.Auth, error)

// schemes is the other half of the registry in domain.AuthSchemes: a scheme is offered to the
// window if it is in that table, and it does something if it is in this one.
var schemes = map[domain.AuthType]scheme{
	domain.AuthBearer: {put: bearer, absorb: absorbBearer},
	domain.AuthBasic:  {put: basic},
	domain.AuthAPIKey: {put: apiKey, absorb: absorbAPIKey},
	domain.AuthJWT:    {put: jwtBearer},
	domain.AuthOAuth2: {fetch: oauthToken},
	domain.AuthDigest: {put: digest},
	domain.AuthAWS:    {put: awsSignature},
}

// encodingBase64 is the answer to a field that says the secret is not written out as it stands.
const encodingBase64 = "base64"

// bearer carries a token somebody else issued. The prefix is a field rather than a constant because
// not every server says «Bearer», and the one that does is not this package's to decide.
func bearer(auth domain.Auth, _ domain.AuthRequest) (domain.AuthOutput, error) {
	token := auth.Answer("token")
	if strings.TrimSpace(token) == "" {
		return domain.AuthOutput{}, nil
	}
	return editable(header("Authorization", withPrefix(auth.Answer("prefix"), token))), nil
}

// absorbBearer reads the token back out of the header it was written into. The prefix is left where
// it is: the row is the whole value, and an edit that dropped the prefix would be an edit to a
// field nobody was looking at.
func absorbBearer(auth domain.Auth, _ domain.RowKind, _, value string) (domain.Auth, error) {
	return auth.With("token", withoutPrefix(auth.Answer("prefix"), value)), nil
}

// basic is the credential of RFC 7617: the two halves joined by a colon and the pair encoded. The
// colon is the split, so a password may hold one and a login may not.
//
// There is no absorb beside it: base64 does not come apart into the two things that went into it,
// so an edit is always an edit to a field and never a guess at one.
func basic(auth domain.Auth, _ domain.AuthRequest) (domain.AuthOutput, error) {
	username, password := auth.Answer("username"), auth.Answer("password")
	if username == "" && password == "" {
		return domain.AuthOutput{}, nil
	}
	joined := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return header("Authorization", "Basic "+joined), nil
}

// apiKey is a credential the server named itself, which is why both halves are fields: nothing here
// knows it is called X-API-Key. Where it goes is a field too — the same key is a header to one
// server and a query parameter to the next.
func apiKey(auth domain.Auth, _ domain.AuthRequest) (domain.AuthOutput, error) {
	name := strings.TrimSpace(auth.Answer("key"))
	value := auth.Answer("value")
	if name == "" || value == "" {
		return domain.AuthOutput{}, nil
	}
	pair := domain.HeaderPair{Name: name, Value: value}
	if auth.Answer("place") == domain.PlaceQuery {
		return domain.AuthOutput{Query: []domain.HeaderPair{pair}, Editable: true}, nil
	}
	return domain.AuthOutput{Headers: []domain.HeaderPair{pair}, Editable: true}, nil
}

func absorbAPIKey(
	auth domain.Auth,
	target domain.RowKind,
	name, value string,
) (domain.Auth, error) {
	if target == domain.RowParams {
		// A query parameter still holding the key's value is that value being edited; anything else is
		// the user renaming the key.
		if name == strings.TrimSpace(auth.Answer("key")) {
			return auth.With("value", value), nil
		}
		return auth.With("key", name).With("value", value), nil
	}
	if name != strings.TrimSpace(auth.Answer("key")) {
		return auth.With("key", name), nil
	}
	return auth.With("value", value), nil
}

// digest hands the engine the credential and nothing else: a Digest request carries a hash of the
// password with a nonce the server has not sent yet. The engine answers the challenge and sends
// again — the one scheme that cannot be finished before that conversation.
//
// There is no absorb: the row is a hash of a challenge nobody here can see.
func digest(auth domain.Auth, _ domain.AuthRequest) (domain.AuthOutput, error) {
	username, password := auth.Answer("username"), auth.Answer("password")
	if username == "" && password == "" {
		return domain.AuthOutput{}, nil
	}
	return domain.AuthOutput{
		Digest: &domain.DigestCredentials{Username: username, Password: password},
	}, nil
}

func header(name, value string) domain.AuthOutput {
	return domain.AuthOutput{Headers: []domain.HeaderPair{{Name: name, Value: value}}}
}

func editable(out domain.AuthOutput) domain.AuthOutput {
	out.Editable = true
	return out
}

// withPrefix and withoutPrefix are a pair on purpose: taking the prefix off one way and putting it
// back another would turn an edit into a value nobody typed.
func withPrefix(prefix, value string) string {
	if prefix = strings.TrimSpace(prefix); prefix == "" {
		return value
	}
	return prefix + " " + value
}

func withoutPrefix(prefix, value string) string {
	if prefix = strings.TrimSpace(prefix); prefix != "" {
		if rest, found := strings.CutPrefix(value, prefix+" "); found {
			return rest
		}
	}
	return value
}
