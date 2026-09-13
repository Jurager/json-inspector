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

// Spec is one request to send.
type Spec struct {
	// ID names this attempt for cancellation. Empty means one is minted; a caller that wants to
	// cancel a particular request supplies its own.
	ID      string
	Method  string
	URL     string
	Headers []domain.HeaderPair
	Body    string
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

	req, err := http.NewRequestWithContext(reqCtx, spec.Method, spec.URL, strings.NewReader(spec.Body))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if e.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", e.cfg.UserAgent)
	}
	for _, h := range spec.Headers {
		if strings.TrimSpace(h.Name) != "" {
			req.Header.Add(h.Name, h.Value)
		}
	}

	resp, err := e.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			res.Cancelled = true
			res.Error = "запрос отменён"
		} else {
			res.Error = err.Error()
		}
		return res
	}
	defer resp.Body.Close()

	data, truncated, err := readCapped(resp.Body, e.cfg.MaxBodyBytes)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	end := time.Now()

	// A connection reused from the pool legitimately skips DNS and connect, so TLS is what the wait
	// is measured from when it happened, and the connect when it did not.
	ready := tlsDone
	if ready.IsZero() {
		ready = connDone
	}

	res.DurationMs = end.Sub(start).Milliseconds()
	res.DNSMs = diffMs(dnsStart, dnsDone)
	res.ConnectMs = diffMs(connStart, connDone)
	res.TLSMs = diffMs(tlsStart, tlsDone)
	res.WaitMs = diffMs(ready, firstByte)
	res.DownloadMs = diffMs(firstByte, end)
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
