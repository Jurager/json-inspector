// Package accountclient is the other side of the wire: the OIDC dance a device signs in through,
// and the account API it speaks afterwards.
//
// Nothing here decides anything. When to sign in, what a refusal means, and what the app keeps are
// the account use case's business; this package knows how to say it in the protocol — a form POST
// with the right fields, a gRPC call with a token in its metadata — and how to turn what comes back
// into the app's own words.
//
// It talks to *a* server and not to ours: the address is an argument of every call, and everything
// it needs about that server it reads from the discovery document, the way any OIDC client does. A
// self-hoster pointing this app at their own provider gets the same app.
package accountclient

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	accountv1 "github.com/Jurager/json-inspector-proto/go/account/v1"

	"json-inspector/internal/domain"
)

// scopes is what the app asks for. `openid` is the identity, `email` is the address, and
// `offline_access` is the one that asks to stay signed in — without it the server hands over an
// access token and no refresh token, and the app would be signed out within the hour.
var scopes = []string{"openid", "email", "offline_access"}

// clientID is the name this app signs in under, and it is not the slug: the slug names the
// program's files on disk, while this is half of a protocol whose other half is the server's own
// list of clients. A device that introduces itself under any other name is turned away before a
// code is printed, and the person sees a refusal where the code should be.
const clientID = "json-inspector-app"

// Client is the OIDC client and the API client in one, because they are one server: the tokens the
// first one buys are what the second one spends.
type Client struct {
	http *http.Client

	mu sync.Mutex
	// discovered is what each server said about itself. A person runs this app against one server for
	// years; asking the same document again on every call would be asking a question whose answer
	// does not change.
	discovered map[string]oauth2.Endpoint
	// pending is the device codes in flight, by the short code a person is looking at: the long one is
	// a secret, and it stays here rather than travelling to a window that would only draw it.
	pending map[string]pending
	// connections are the gRPC channels, one per server.
	connections map[string]*grpc.ClientConn
}

// pending is a device code waiting for somebody: the secret, where it was asked for, and how often
// the app may ask again.
type pending struct {
	deviceCode string
	address    string
	interval   time.Duration
	expiresAt  time.Time
}

func New(httpClient *http.Client) *Client {
	return &Client{
		http:        httpClient,
		discovered:  map[string]oauth2.Endpoint{},
		pending:     map[string]pending{},
		connections: map[string]*grpc.ClientConn{},
	}
}

// Close lets go of the channels. Nothing depends on it — the process is ending anyway — but a
// connection that outlives its use is a connection somebody will wonder about.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for address, conn := range c.connections {
		_ = conn.Close()
		delete(c.connections, address)
	}
}

// Authorize asks the server for a device code: what the person confirms, and where.
//
// The device's own name travels as headers on this one request. The protocol has no field for it —
// by the standard a device says which client it is and nothing about itself — and a name the person
// reads later in their list of devices has to come from somewhere.
func (c *Client) Authorize(ctx context.Context, address string, device domain.Device) (
	domain.Challenge, error) {
	endpoints, err := c.endpoints(ctx, address)
	if err != nil {
		return domain.Challenge{}, err
	}

	form := url.Values{
		"client_id": {clientID},
		"scope":     {strings.Join(scopes, " ")},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoints.DeviceAuthURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return domain.Challenge{}, refuse(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	setDevice(request, device)

	var asked struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
		Error                   string `json:"error"`
	}
	if err := c.do(request, &asked); err != nil {
		return domain.Challenge{}, err
	}
	if asked.Error != "" || asked.DeviceCode == "" || asked.UserCode == "" {
		return domain.Challenge{}, domain.Refuse(domain.CodeServerRefused, nil, nil)
	}

	// The code travels in the address when the server offers that: the person then confirms without
	// typing anything, which is the whole point of the short code.
	where := asked.VerificationURIComplete
	if where == "" {
		where = asked.VerificationURI
	}
	expires := time.Duration(asked.ExpiresIn) * time.Second

	c.remember(asked.UserCode, pending{
		deviceCode: asked.DeviceCode,
		address:    address,
		interval:   interval(asked.Interval),
		expiresAt:  time.Now().Add(expires),
	})

	return domain.Challenge{UserCode: asked.UserCode, URL: where, ExpiresAt: time.Now().Add(expires)},
		nil
}

// Tokens waits for the person to confirm the code, and answers the pair of tokens when they do.
//
// Waiting is the standard's polling, and the interval is the server's own: asking faster is what
// the standard calls `slow_down`, and it is a client burning somebody's CPU to learn nothing new.
func (c *Client) Tokens(ctx context.Context, address string, challenge domain.Challenge) (
	domain.Tokens, error) {
	asked, err := c.take(challenge.UserCode)
	if err != nil {
		return domain.Tokens{}, err
	}
	endpoints, err := c.endpoints(ctx, address)
	if err != nil {
		return domain.Tokens{}, err
	}

	form := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {asked.deviceCode},
		"client_id":   {clientID},
	}

	for {
		select {
		case <-ctx.Done():
			return domain.Tokens{}, ctx.Err()
		case <-time.After(asked.interval):
		}

		tokens, pending, err := c.token(ctx, endpoints.TokenURL, form)
		if err != nil {
			return domain.Tokens{}, err
		}
		if !pending {
			return tokens, nil
		}
		if time.Now().After(asked.expiresAt) {
			// The code ran out while the app was asking: the person took too long, and the next thing
			// they do is start again.
			return domain.Tokens{}, domain.Refuse(domain.CodeSignInFailed, domain.ErrNotAllowed, nil)
		}
	}
}

// Refresh trades the refresh token for a new pair. It is the one call that happens without anybody
// watching, which is why a refusal here has to be told apart from the server being away: the first
// means the account is gone, and the second means try later.
func (c *Client) Refresh(ctx context.Context, address, refresh string) (domain.Tokens, error) {
	endpoints, err := c.endpoints(ctx, address)
	if err != nil {
		return domain.Tokens{}, err
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refresh},
		"client_id":     {clientID},
	}
	tokens, _, err := c.token(ctx, endpoints.TokenURL, form)
	return tokens, err
}

// token posts to the token endpoint and reads what comes back. The second answer says whether the
// server is still waiting for somebody — which is not a failure, it is the middle of a sign-in.
func (c *Client) token(ctx context.Context, endpoint string, form url.Values) (
	domain.Tokens, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return domain.Tokens{}, false, refuse(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	var answer struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int    `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	// The status is read past on purpose, and this is the only call that does: the token endpoint says
	// `authorization_pending` with a 400, so a client that stopped at the status would give up on the
	// first poll — which is to say, always, and before anybody had a chance to open their mail.
	if _, err := c.read(request, &answer); err != nil {
		return domain.Tokens{}, false, err
	}

	switch answer.Error {
	case "":
	case "authorization_pending":
		// Nothing has happened yet, which is the ordinary answer while somebody is reading their mail.
		return domain.Tokens{}, true, nil
	case "slow_down":
		return domain.Tokens{}, true, nil
	default:
		// `access_denied`, `expired_token`, `invalid_grant` — the sign-in is over, and it did not
		// produce anything.
		return domain.Tokens{}, false,
			domain.Refuse(domain.CodeSignInFailed, domain.ErrNotAllowed, nil)
	}

	if answer.AccessToken == "" {
		return domain.Tokens{}, false, domain.Refuse(domain.CodeServerRefused, nil, nil)
	}
	return domain.Tokens{
		Access:    answer.AccessToken,
		Refresh:   answer.RefreshToken,
		ExpiresAt: time.Now().Add(time.Duration(answer.ExpiresIn) * time.Second),
	}, false, nil
}

// endpoints reads the server's discovery document, once per address.
func (c *Client) endpoints(ctx context.Context, address string) (oauth2.Endpoint, error) {
	c.mu.Lock()
	found, ok := c.discovered[address]
	c.mu.Unlock()
	if ok {
		return found, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(address, "/")+"/.well-known/openid-configuration", nil)
	if err != nil {
		return oauth2.Endpoint{}, refuse(err)
	}
	request.Header.Set("Accept", "application/json")

	var document struct {
		DeviceAuthorizationEndpoint string   `json:"device_authorization_endpoint"`
		TokenEndpoint               string   `json:"token_endpoint"`
		GrantTypes                  []string `json:"grant_types_supported"`
	}
	if err := c.do(request, &document); err != nil {
		return oauth2.Endpoint{}, err
	}

	// A server that does not offer the device flow is not a server this app can sign in to — and
	// saying so now is kinder than failing three steps later with a code nobody understands.
	if document.DeviceAuthorizationEndpoint == "" || document.TokenEndpoint == "" {
		return oauth2.Endpoint{}, domain.Refuse(domain.CodeServerRefused, nil, nil)
	}

	endpoints := oauth2.Endpoint{
		DeviceAuthURL: document.DeviceAuthorizationEndpoint,
		TokenURL:      document.TokenEndpoint,
	}
	c.mu.Lock()
	c.discovered[address] = endpoints
	c.mu.Unlock()
	return endpoints, nil
}

// do sends a request and reads a JSON answer, turning what happened into the app's own refusals.
//
// The distinction it makes is the one everything above depends on: a server that answered is a
// server that has an opinion, and a server that did not answer is the weather. Both are ordinary,
// and neither is a bug in the app.
func (c *Client) do(request *http.Request, into any) error {
	status, err := c.read(request, into)
	if err != nil {
		return err
	}
	if status >= http.StatusBadRequest {
		return domain.Refuse(domain.CodeServerRefused, nil,
			domain.Args{"status": fmt.Sprintf("%d", status)})
	}
	return nil
}

// read makes the call and fills what was asked for, and answers the status as well — because the
// status is not always the verdict. Everything but the token endpoint wants the old behaviour and
// calls do; that one has to see the body of a 400 to know whether the sign-in is still going on.
func (c *Client) read(request *http.Request, into any) (int, error) {
	response, err := c.http.Do(request)
	if err != nil {
		return 0, domain.Refuse(domain.CodeServerUnreachable, nil, nil)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return response.StatusCode, domain.Refuse(domain.CodeServerUnreachable, nil, nil)
	}
	if err := json.Unmarshal(body, into); err != nil {
		// Something answered, and it was not this protocol: a web server, a proxy's error page, a
		// wrong port.
		return response.StatusCode, domain.Refuse(domain.CodeServerRefused, nil, nil)
	}
	return response.StatusCode, nil
}

func (c *Client) remember(userCode string, flow pending) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Anything that ran out is dropped while we are here: a map that only grows is a map that holds
	// codes nobody will ever ask about again.
	for code, held := range c.pending {
		if time.Now().After(held.expiresAt) {
			delete(c.pending, code)
		}
	}
	c.pending[userCode] = flow
}

func (c *Client) take(userCode string) (pending, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	flow, ok := c.pending[userCode]
	if !ok {
		return pending{}, domain.Refuse(domain.CodeSignInFailed, domain.ErrNotFound, nil)
	}
	delete(c.pending, userCode)
	return flow, nil
}

// interval is how long to wait between two questions. A server that named no interval, or one that
// is refusing to name a usable one, gets the standard's own floor of five seconds: a client that
// hammers is a client that gets told to slow down.
func interval(seconds int) time.Duration {
	if seconds < 1 {
		return 5 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func setDevice(request *http.Request, device domain.Device) {
	request.Header.Set("X-Device-Name", device.Name)
	request.Header.Set("X-Device-Platform", device.Platform)
	request.Header.Set("X-Device-Version", device.AppVersion)
}

// dial is the gRPC channel to a server, opened once and kept: the API is called while a person is
// looking at the window, and a fresh handshake per click is a click that feels slow.
func (c *Client) dial(address string) (*grpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if conn, ok := c.connections[address]; ok {
		return conn, nil
	}

	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" {
		return nil, domain.Refuse(domain.CodeServerRefused, nil, nil)
	}

	options := []grpc.DialOption{}
	if parsed.Scheme == "https" {
		options = append(options, grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	} else {
		options = append(options, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(parsed.Host, options...)
	if err != nil {
		return nil, domain.Refuse(domain.CodeServerRefused, nil, nil)
	}
	c.connections[address] = conn
	return conn, nil
}

// call is one API request: the channel, the token in the metadata, and the refusal it came back
// with.
func call[T any](c *Client, ctx context.Context, address, access string,
	do func(context.Context, accountv1.AccountServiceClient) (T, error)) (T, error) {
	var nothing T

	conn, err := c.dial(address)
	if err != nil {
		return nothing, err
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+access)

	answer, err := do(ctx, accountv1.NewAccountServiceClient(conn))
	if err != nil {
		return nothing, fromStatus(err)
	}
	return answer, nil
}
