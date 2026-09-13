package httpx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

func newTestEngine(t *testing.T, cfg Config) *Engine {
	t.Helper()
	return NewEngine(cfg, platform.NewIDGen())
}

// send is the shortest way to make one request from a spec: a method, a URL, and headers as a map.
// A map has no order, so it is sorted here — a test that cares about the order passes pairs itself.
func send(engine *Engine, method, url string, headers map[string]string, body string) *domain.Response {
	return engine.Do(context.Background(), Spec{
		Method:  method,
		URL:     url,
		Headers: domain.PairsFromMap(headers),
		Body:    body,
	})
}

// inflight is how many attempts the engine still holds. It is what a cancelled request must leave
// behind, and the tests are in this package precisely so they can look.
func inflight(engine *Engine) int {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return len(engine.inflight)
}

// TestTimings adds up: every phase is inside the total, and a plaintext server has no TLS phase at
// all — the numbers the response viewer draws have to mean something.
func TestTimings(t *testing.T) {
	t.Run("plain http", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(20 * time.Millisecond)
			_, _ = w.Write([]byte("ok"))
		}))
		defer srv.Close()

		res := send(newTestEngine(t, Config{}), http.MethodGet, srv.URL, nil, "")
		if res.Error != "" {
			t.Fatalf("send failed: %s", res.Error)
		}
		if res.TLSUs != nil {
			t.Errorf("tlsUs = %d on a plaintext server, want no phase at all", *res.TLSUs)
		}
		// Not asserted as positive: a connect to localhost takes a fraction of a millisecond.
		if res.ConnectUs == nil || *res.ConnectUs > res.DurationUs {
			t.Errorf("connectUs = %v, want a phase inside the %dus total", res.ConnectUs, res.DurationUs)
		}
		// The wait starts when the connection is ready — there is no TLS handshake to wait for.
		if res.WaitUs == nil || *res.WaitUs < 20_000 {
			t.Errorf("waitUs = %v, want at least the 20ms the handler sleeps", res.WaitUs)
		}
		assertPhasesWithinTotal(t, res)
	})

	t.Run("tls", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("ok"))
		}))
		defer srv.Close()

		res := send(newTestEngine(t, Config{SkipTLSVerify: true}), http.MethodGet, srv.URL, nil, "")
		if res.Error != "" {
			t.Fatalf("send failed: %s", res.Error)
		}
		if res.TLSUs == nil || *res.TLSUs <= 0 {
			t.Errorf("tlsUs = %v, want the handshake measured", res.TLSUs)
		}
		assertPhasesWithinTotal(t, res)
	})

	// The default: every send dials, even a repeat. A probe that reused a connection would report a
	// breakdown that depends on when it was last used, and the same request sent twice has to give
	// the same one.
	t.Run("a repeat dials again", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(20 * time.Millisecond)
			_, _ = w.Write([]byte("ok"))
		}))
		defer srv.Close()

		engine := newTestEngine(t, Config{})
		if res := send(engine, http.MethodGet, srv.URL, nil, ""); res.Error != "" {
			t.Fatalf("first send failed: %s", res.Error)
		}
		second := send(engine, http.MethodGet, srv.URL, nil, "")

		if second.Error != "" {
			t.Fatalf("second send failed: %s", second.Error)
		}
		if second.ConnectUs == nil {
			t.Error("connectUs is absent on a repeat, want the connection dialled again")
		}
		// The other two are absent for reasons of their own, and both are the same kind of absence:
		// the server is an IP literal, so there is no name to look up, and it speaks plain HTTP, so
		// there is no handshake to perform.
		if second.DNSUs != nil || second.TLSUs != nil {
			t.Errorf("dns/tls = %v/%v against a literal address over plain http, want none",
				second.DNSUs, second.TLSUs)
		}
		if second.WaitUs == nil || *second.WaitUs < 20_000 {
			t.Errorf("waitUs = %v, want at least the 20ms the handler sleeps", second.WaitUs)
		}
		assertPhasesWithinTotal(t, second)
	})
}

// With connection reuse asked for, a repeat keeps the connection: it has no connect phase, which is
// exactly what the timings tab reads to say so. That is the half of the behaviour a setting will
// one day switch between.
func TestKeepingConnectionsReusesThem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	engine := newTestEngine(t, Config{KeepConnections: true})
	if res := send(engine, http.MethodGet, srv.URL, nil, ""); res.Error != "" {
		t.Fatalf("first send failed: %s", res.Error)
	}
	second := send(engine, http.MethodGet, srv.URL, nil, "")

	if second.Error != "" {
		t.Fatalf("second send failed: %s", second.Error)
	}
	if second.ConnectUs != nil {
		t.Errorf("connectUs = %v on a reused connection, want none — nothing was dialled", *second.ConnectUs)
	}
	if second.WaitUs == nil {
		t.Error("waitUs is absent: the request did happen, only the dial did not")
	}
}

// A request that never got an answer has no phases to show, and says so by having none — the five
// zeroes that look like measurements are the thing this replaced.
func TestAFailedRequestHasNoPhases(t *testing.T) {
	// A port nothing listens on: the connect fails, so there is no first byte to measure from.
	res := send(newTestEngine(t, Config{Timeout: 2 * time.Second}), http.MethodGet, "http://127.0.0.1:1/", nil, "")

	if res.Error == "" {
		t.Fatal("a request to a dead port reported no error")
	}
	if res.WaitUs != nil || res.DownloadUs != nil {
		t.Errorf("wait/download = %v/%v for a request that never reached a server, want none",
			res.WaitUs, res.DownloadUs)
	}
	// The total is not asserted as positive: a refused connection on the loopback takes less than
	// the microsecond the total is reported in.
	if res.DurationUs < 0 {
		t.Errorf("durationUs = %d, want the attempt timed", res.DurationUs)
	}
}

func assertPhasesWithinTotal(t *testing.T, res *domain.Response) {
	t.Helper()
	// Only the phases that happened are added up: an absent one was not a zero-length phase, it was
	// no phase at all, and the total already accounts for the request as a whole.
	var sum int64
	for _, us := range []*int64{res.DNSUs, res.ConnectUs, res.TLSUs, res.WaitUs, res.DownloadUs} {
		if us != nil {
			sum += *us
		}
	}
	if sum > res.DurationUs {
		t.Errorf("phases sum to %dus, more than the %dus total", sum, res.DurationUs)
	}
	if res.DurationUs <= 0 {
		t.Errorf("durationUs = %d, want a positive total", res.DurationUs)
	}
}

// TestTLSVerificationIsOnByDefault is the difference between "the user's endpoint has a
// self-signed certificate" and "we stopped checking certificates for everyone".
func TestTLSVerificationIsOnByDefault(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	res := send(newTestEngine(t, Config{}), http.MethodGet, srv.URL, nil, "")
	if res.Error == "" {
		t.Error("a self-signed certificate was accepted with verification on")
	}

	relaxed := send(newTestEngine(t, Config{SkipTLSVerify: true}), http.MethodGet, srv.URL, nil, "")
	if relaxed.Error != "" {
		t.Errorf("SkipTLSVerify did not take effect: %s", relaxed.Error)
	}
}

// TestRedirects covers both halves: the default follows, and the setting stops at the first
// response so a caller can see a 3xx for itself.
func TestRedirects(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("final"))
	}))
	defer final.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer first.Close()

	t.Run("followed by default", func(t *testing.T) {
		res := send(newTestEngine(t, Config{}), http.MethodGet, first.URL, nil, "")
		if res.Status != http.StatusOK || res.Body != "final" {
			t.Errorf("got %d %q, want the redirect target", res.Status, res.Body)
		}
	})

	t.Run("stopped when asked", func(t *testing.T) {
		res := send(newTestEngine(t, Config{NoRedirects: true}), http.MethodGet, first.URL, nil, "")
		if res.Status != http.StatusFound {
			t.Errorf("status = %d, want the 302 itself", res.Status)
		}
		if got := res.HeaderValues("Location"); len(got) != 1 || got[0] != final.URL {
			t.Errorf("Location = %v, want the target", got)
		}
	})

	t.Run("a loop ends with the redirect limit", func(t *testing.T) {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, srv.URL, http.StatusFound)
		}))
		defer srv.Close()

		res := send(newTestEngine(t, Config{MaxRedirects: 3}), http.MethodGet, srv.URL, nil, "")
		if !strings.Contains(res.Error, "3 redirects") {
			t.Errorf("error = %q, want the redirect limit named", res.Error)
		}
	})
}

// TestRepeatedHeadersBothSurvive is the bug the sequence-shaped headers exist for: a map would keep
// the first Set-Cookie and drop the rest, and cookies would go missing with no error anywhere.
func TestRepeatedHeadersBothSurvive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "a=1")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Add("Set-Cookie", "c=3")
		w.Header().Set("X-One", "1")
	}))
	defer srv.Close()

	res := send(newTestEngine(t, Config{}), http.MethodGet, srv.URL, nil, "")
	cookies := res.HeaderValues("Set-Cookie")
	if len(cookies) != 3 {
		t.Fatalf("Set-Cookie = %v, want all three", cookies)
	}
	for i, want := range []string{"a=1", "b=2", "c=3"} {
		if cookies[i] != want {
			t.Errorf("cookie %d = %q, want %q", i, cookies[i], want)
		}
	}
	// Sorted, so two sends of the same response list the headers in the same order.
	if len(res.Headers) != 6 {
		t.Errorf("headers = %v, want every line kept", res.Headers)
	}
	names := make([]string, 0, len(res.Headers))
	for _, h := range res.Headers {
		names = append(names, h.Name)
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("header names %v are not sorted, so the order would vary between sends", names)
	}
}

// TestBodyCap truncates instead of buffering whatever the server sends, and says so.
func TestBodyCap(t *testing.T) {
	const cap int64 = 1024
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", int(cap)+500)))
	}))
	defer srv.Close()

	res := send(newTestEngine(t, Config{MaxBodyBytes: cap}), http.MethodGet, srv.URL, nil, "")
	if !res.BodyTruncated {
		t.Error("bodyTruncated = false, want the cap reported")
	}
	if int64(len(res.Body)) != cap {
		t.Errorf("body length = %d, want exactly the %d-byte cap", len(res.Body), cap)
	}

	small := send(newTestEngine(t, Config{}), http.MethodGet, srv.URL, nil, "")
	if small.BodyTruncated {
		t.Error("a body under the default cap was reported as truncated")
	}
	if int64(len(small.Body)) != cap+500 {
		t.Errorf("body length = %d, want the whole body", len(small.Body))
	}
}

// TestCancelStopsOneRequest checks the per-id cancellation: only the request named is stopped, and
// the engine forgets it afterwards.
func TestCancelStopsOneRequest(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte("late"))
	}))
	defer srv.Close()
	defer close(release)

	engine := newTestEngine(t, Config{})

	done := make(chan *domain.Response, 1)
	go func() {
		done <- engine.Do(context.Background(), Spec{ID: "slow", Method: http.MethodGet, URL: srv.URL})
	}()

	// Wait until the request is registered, then cancel it.
	deadline := time.Now().Add(2 * time.Second)
	for {
		engine.mu.Lock()
		_, running := engine.inflight["slow"]
		engine.mu.Unlock()
		if running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the request never registered itself")
		}
		time.Sleep(time.Millisecond)
	}

	if !engine.Cancel("slow") {
		t.Fatal("Cancel reported nothing in flight")
	}

	select {
	case res := <-done:
		if !res.Cancelled || res.Error == "" {
			t.Errorf("res = %+v, want it marked cancelled with a reason", res)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the cancelled request never returned")
	}

	if engine.Cancel("slow") {
		t.Error("the id is still in flight after the request returned")
	}
	if n := inflight(engine); n != 0 {
		t.Errorf("%d attempts are still in flight, want none", n)
	}
}

// TestConcurrentRequestsCancelIndependently is what the single global cancel could not do.
func TestConcurrentRequestsCancelIndependently(t *testing.T) {
	release := make(chan struct{})
	var started sync.WaitGroup
	started.Add(3)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started.Done()
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
	}))
	defer srv.Close()
	defer close(release)

	engine := newTestEngine(t, Config{})

	results := make(chan string, 3)
	for _, id := range []string{"a", "b", "c"} {
		go func(id string) {
			res := engine.Do(context.Background(), Spec{ID: id, Method: http.MethodGet, URL: srv.URL})
			if res.Cancelled {
				results <- id + ":cancelled"
				return
			}
			results <- id + ":answered"
		}(id)
	}

	started.Wait()
	if !engine.Cancel("b") {
		t.Fatal("cancel b found nothing in flight")
	}
	// b is still registered: an attempt leaves the registry when it returns, not when it is
	// cancelled, and cancelling twice is harmless.
	if n := inflight(engine); n != 3 {
		t.Fatalf("%d attempts are in flight, want all three", n)
	}
	if !engine.Cancel("a") || !engine.Cancel("c") {
		t.Fatal("an attempt could not be cancelled on its own")
	}

	got := map[string]bool{}
	for i := 0; i < 3; i++ {
		select {
		case r := <-results:
			got[r] = true
		case <-time.After(3 * time.Second):
			t.Fatal("a request never returned")
		}
	}
	for _, id := range []string{"a", "b", "c"} {
		if !got[id+":cancelled"] {
			t.Errorf("%s was not cancelled: %v", id, got)
		}
	}
}

// TestCookieJarIsOptIn keeps the old behaviour the default and shows what turning it on buys.
func TestCookieJarIsOptIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err == nil {
			_, _ = fmt.Fprintf(w, "cookie=%s", c.Value)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
		_, _ = w.Write([]byte("set"))
	}))
	defer srv.Close()

	t.Run("off by default", func(t *testing.T) {
		engine := newTestEngine(t, Config{})
		send(engine, http.MethodGet, srv.URL, nil, "")
		second := send(engine, http.MethodGet, srv.URL, nil, "")
		if second.Body != "set" {
			t.Errorf("second body = %q, want no cookie carried", second.Body)
		}
	})

	t.Run("on when asked", func(t *testing.T) {
		engine := newTestEngine(t, Config{UseCookieJar: true})
		send(engine, http.MethodGet, srv.URL, nil, "")
		second := send(engine, http.MethodGet, srv.URL, nil, "")
		if second.Body != "cookie=abc" {
			t.Errorf("second body = %q, want the cookie carried", second.Body)
		}
	})
}

// TestProxyIsUsed puts a proxy in the way and checks the request really goes through it.
func TestProxyIsUsed(t *testing.T) {
	var seen atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("direct"))
	}))
	defer target.Close()

	parsed, err := url.Parse(target.URL)
	if err != nil {
		t.Fatalf("parsing the target URL: %v", err)
	}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Add(1)
		httputil.NewSingleHostReverseProxy(parsed).ServeHTTP(w, r)
	}))
	defer proxy.Close()

	res := send(newTestEngine(t, Config{ProxyURL: proxy.URL}), http.MethodGet, target.URL, nil, "")
	if res.Error != "" {
		t.Fatalf("send through the proxy failed: %s", res.Error)
	}
	if seen.Load() == 0 {
		t.Error("the proxy never saw the request")
	}
	if res.Body != "direct" {
		t.Errorf("body = %q, want the proxied response", res.Body)
	}
}

// TestRequestHeadersAreSent is the plain contract: what the caller lists is what the server sees.
func TestRequestHeadersAreSent(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}))
	defer srv.Close()

	engine := newTestEngine(t, Config{})
	engine.Do(context.Background(), Spec{
		Method: http.MethodPost,
		URL:    srv.URL,
		Body:   `{"a":1}`,
		Headers: []domain.HeaderPair{
			{Name: "X-Second", Value: "2"},
			{Name: "X-First", Value: "1"},
			{Name: "X-Repeated", Value: "a"},
			{Name: "X-Repeated", Value: "b"},
			{Name: "  ", Value: "dropped"},
		},
	})

	if got.Get("X-First") != "1" || got.Get("X-Second") != "2" {
		t.Errorf("headers did not arrive: %v", got)
	}
	if values := got.Values("X-Repeated"); len(values) != 2 {
		t.Errorf("X-Repeated = %v, want both values", values)
	}
	if got.Get("Content-Length") != strconv.Itoa(len(`{"a":1}`)) {
		t.Errorf("content-length = %q, want the body length", got.Get("Content-Length"))
	}
}

// TestBadURLIsReportedNotPanicked keeps a mistyped URL a message in the response rather than a
// crash in the app.
func TestBadURLIsReportedNotPanicked(t *testing.T) {
	res := send(newTestEngine(t, Config{}), http.MethodGet, "not a url at all", nil, "")
	if res.Error == "" {
		t.Error("a malformed URL produced no error")
	}
	if res.Cancelled || res.Status != 0 {
		t.Errorf("res = %+v, want a plain failure", res)
	}
}
