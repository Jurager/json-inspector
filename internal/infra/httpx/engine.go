// Package httpx is the app's outbound HTTP: the one place a user request leaves the process.
package httpx

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/icholy/digest"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// Engine sends user requests. Each one is registered under an id while it is in flight, so it can
// be cancelled on its own rather than "whichever request happens to be running".
type Engine struct {
	mu       sync.Mutex
	client   *http.Client
	inflight map[string]context.CancelFunc
	cfg      Config
	ids      platform.IDGen
}

func NewEngine(cfg Config, ids platform.IDGen) *Engine {
	cfg = cfg.withDefaults()
	engine := &Engine{
		cfg:      cfg,
		inflight: map[string]context.CancelFunc{},
		ids:      ids,
	}
	engine.client = &http.Client{
		Timeout:       cfg.Timeout,
		Transport:     newTransport(cfg),
		CheckRedirect: redirectPolicy(cfg),
	}
	if cfg.UseCookieJar {
		// A jar outlives a single request, so it is opt-in: the app has behaved as "every request
		// stands alone" until now, and turning it on silently would change what a send does.
		engine.client.Jar, _ = cookiejar.New(nil)
	}
	return engine
}

// Client is the client every user request goes out through, for the one thing that is not a user
// request and still leaves the process the same way: an identity provider's token endpoint. It
// carries the proxy, the timeout and the TLS settings the user configured, which is the whole reason
// to hand it over rather than build a second one that quietly ignores them.
func (e *Engine) Client() *http.Client { return e.client }

// Spec is one request to send.
type Spec struct {
	// ID names this attempt for cancellation. Empty means one is minted; a caller that wants to
	// cancel a particular request supplies its own.
	ID      string
	Method  string
	URL     string
	Headers []domain.HeaderPair
	Body    string
	// Digest is a credential that cannot be put on the request until the server has said how. The
	// first send is what asks — it comes back refused, with the realm and the nonce in the challenge
	// — and the second carries the answer. The two are one attempt: one id, one cancellation, one
	// duration, because that is what they are.
	Digest *domain.DigestCredentials
}

// newRequest builds the request a spec describes. Both halves of Digest are built from the same spec
// and differ only in the header the second one carries, which is why this is a function and not two.
func (e *Engine) newRequest(ctx context.Context, spec Spec) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, spec.Method, complete(spec.URL), strings.NewReader(spec.Body))
	if err != nil {
		return nil, err
	}
	if e.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", e.cfg.UserAgent)
	}
	for _, h := range spec.Headers {
		if strings.TrimSpace(h.Name) != "" {
			req.Header.Add(h.Name, h.Value)
		}
	}
	return req, nil
}

// answerChallenge is the second half of Digest: the server refused the request and said how, and
// what it said becomes the header the retry carries.
//
// The address the answer is computed over is the one the request is actually going to — the engine
// completes a bare host into a scheme and a path, and a response computed over what the user typed
// rather than over what was sent is one the server refuses.
func (e *Engine) answerChallenge(ctx context.Context, spec Spec, refused *http.Response) (*http.Response, []domain.HeaderPair, error) {
	challenge, err := digest.FindChallenge(refused.Header)
	if err != nil {
		return nil, nil, err
	}
	retry, err := e.newRequest(ctx, spec)
	if err != nil {
		return nil, nil, err
	}
	credentials, err := digest.Digest(challenge, digest.Options{
		Method:   spec.Method,
		URI:      retry.URL.RequestURI(),
		GetBody:  retry.GetBody,
		Count:    1,
		Username: spec.Digest.Username,
		Password: spec.Digest.Password,
	})
	if err != nil {
		return nil, nil, err
	}
	answer := credentials.String()
	retry.Header.Set("Authorization", answer)

	sent, err := e.client.Do(retry)
	if err != nil {
		return nil, nil, err
	}
	return sent, []domain.HeaderPair{{Name: "Authorization", Value: answer}}, nil
}

// Do sends the request and reports what came back. A transport failure is not an error return: it
// is a Response carrying Error, because that is what the UI shows either way.
func (e *Engine) Do(ctx context.Context, spec Spec) *domain.Response {
	res := &domain.Response{}
	start := time.Now()

	id := spec.ID
	if id == "" {
		id = e.ids()
	}

	var (
		dnsStart, dnsDone   time.Time
		connStart, connDone time.Time
		tlsStart, tlsDone   time.Time
		firstByte           time.Time
	)
	trace := &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:              func(httptrace.DNSDoneInfo) { dnsDone = time.Now() },
		ConnectStart:         func(_, _ string) { connStart = time.Now() },
		ConnectDone:          func(_, _ string, _ error) { connDone = time.Now() },
		TLSHandshakeStart:    func() { tlsStart = time.Now() },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { tlsDone = time.Now() },
		GotFirstResponseByte: func() { firstByte = time.Now() },
	}

	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	reqCtx = httptrace.WithClientTrace(reqCtx, trace)

	e.mu.Lock()
	e.inflight[id] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.inflight, id)
		e.mu.Unlock()
	}()

	req, err := e.newRequest(reqCtx, spec)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	resp, err := e.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// No text: a cancelled attempt says so with the flag, and the window words it. Writing a
			// sentence here would freeze one language into every record that was ever stopped.
			res.Cancelled = true
		} else {
			res.Error = err.Error()
		}
		return res
	}

	// The refusal is what a Digest request is for: it is where the realm and the nonce come from. What
	// the window is waiting for is the second answer, so the first one is dropped on the floor — and
	// a challenge that cannot be answered leaves the refusal standing, because that is the answer.
	if spec.Digest != nil && resp.StatusCode == http.StatusUnauthorized {
		answered, sent, answerErr := e.answerChallenge(reqCtx, spec, resp)
		if answerErr == nil {
			resp.Body.Close()
			resp = answered
			res.SentHeaders = sent
		}
	}
	defer resp.Body.Close()

	data, truncated, err := readCapped(resp.Body, e.cfg.MaxBodyBytes)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	end := time.Now()

	// A connection reused from the pool skips DNS, connect and TLS — those events never fire — so
	// what the wait is measured from is the last thing that did happen: the handshake, the connect,
	// or, when the connection was already there, the moment the request started. Without that last
	// fallback a repeat to the same host reports nothing but the total, which reads as a breakdown
	// that failed to be measured rather than as a request that spent its time waiting.
	ready := tlsDone
	if ready.IsZero() {
		ready = connDone
	}
	if ready.IsZero() {
		ready = start
	}

	res.DurationUs = end.Sub(start).Microseconds()
	res.DNSUs = diffUs(dnsStart, dnsDone)
	res.ConnectUs = diffUs(connStart, connDone)
	res.TLSUs = diffUs(tlsStart, tlsDone)
	res.WaitUs = diffUs(ready, firstByte)
	res.DownloadUs = diffUs(firstByte, end)
	res.Status = resp.StatusCode
	res.StatusText = resp.Status
	res.ContentType = resp.Header.Get("Content-Type")
	res.Headers = headerPairs(resp.Header)
	res.Body = string(data)
	res.BodyTruncated = truncated
	return res
}

// Cancel stops one request by id and reports whether anything was in flight under it.
func (e *Engine) Cancel(id string) bool {
	e.mu.Lock()
	cancel, ok := e.inflight[id]
	e.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// readCapped reads a body up to a limit and says whether there was more. The extra byte is what
// distinguishes a body that ends exactly at the cap from one that was cut there.
func readCapped(body io.Reader, limit int64) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return data[:limit], true, nil
	}
	return data, false, nil
}

// headerPairs flattens the response headers, keeping every value of a repeated name. The names are
// sorted because http.Header is a map: without this the Headers tab would list them in a different
// order on every send.
func headerPairs(header http.Header) []domain.HeaderPair {
	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]domain.HeaderPair, 0, len(names))
	for _, name := range names {
		for _, value := range header[name] {
			out = append(out, domain.HeaderPair{Name: name, Value: value})
		}
	}
	return out
}
