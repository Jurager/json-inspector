// Package httpx is the app's outbound HTTP: the one place a user request leaves the process.
package httpx

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"json-inspector/internal/domain"
)

// Engine sends user requests and keeps the handle needed to cancel them.
//
// Today it is the shape the app had: one request in flight, a client built per call with no
// Transport of its own, and a single cancel handle. The per-request ids, the transport options
// (proxy, TLS, redirects, cookie jar) and the body cap land with the request feature; the methods
// the use cases call do not change when they do.
type Engine struct {
	mu        sync.Mutex
	cancelReq context.CancelFunc
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Send(method, url string, headers map[string]string, body string) *domain.Response {
	return e.sendTimed(method, url, headers, body)
}

func (e *Engine) Fetch(url string, headers map[string]string) *domain.Response {
	return e.sendTimed(http.MethodGet, url, headers, "")
}

func (e *Engine) Cancel() {
	e.mu.Lock()
	cancel := e.cancelReq
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// The timed round trip behind Send and Fetch: the client trace is what fills in the per-phase
// timings the response viewer draws.
func (e *Engine) sendTimed(method, url string, headers map[string]string, body string) *domain.Response {
	res := &domain.Response{}
	start := time.Now()

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

	ctx, cancel := context.WithCancel(context.Background())
	ctx = httptrace.WithClientTrace(ctx, trace)
	e.mu.Lock()
	e.cancelReq = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.cancelReq = nil
		e.mu.Unlock()
	}()

	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	for name, value := range headers {
		if strings.TrimSpace(name) != "" {
			req.Header.Add(name, value)
		}
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
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

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	end := time.Now()

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
	res.Headers = make(map[string]string, len(resp.Header))
	for k, vs := range resp.Header {
		if len(vs) > 0 {
			res.Headers[k] = vs[0]
		}
	}
	res.Body = string(data)
	return res
}
