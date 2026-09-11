package main

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

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"json-inspector/internal/bridge"
	"json-inspector/internal/jsonapi"
	"json-inspector/internal/update"
)

// App is the Wails application.
// Its exported methods are callable from the frontend through the generated bindings.
type App struct {
	ctx context.Context

	mu        sync.Mutex
	cancelReq context.CancelFunc

	// bridge is the loopback WS server the extension connects to; it lets the
	// app push control messages back (pause) in the opposite direction.
	bridge *bridge.Server
}

// setBridge wires the bridge server so PauseCapture can reach the extension.
func (a *App) setBridge(s *bridge.Server) {
	a.bridge = s
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved so runtime
// methods (events) can be called later.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startUpdateCheck()
}

// startUpdateCheck runs the once-a-day background version check and tells the
// UI when a newer version is available.
func (a *App) startUpdateCheck() {
	go func() {
		if u := update.StartupCheck(); u != nil {
			runtime.EventsEmit(a.ctx, "update-available", u)
		}
	}()
}

// Version returns the running version.
func (a *App) Version() string {
	return update.Current
}

// ToggleMaximize toggles the window zoom (native macOS double-click on the
// title bar).
func (a *App) ToggleMaximize() {
	runtime.WindowToggleMaximise(a.ctx)
}

// ShowAbout emits the event that opens the "About" dialog in the UI.
func (a *App) ShowAbout() {
	runtime.EventsEmit(a.ctx, "show-about")
}

// handleUrlOpen is called when the app is opened via the json-inspector://
// custom URL scheme (e.g. the Chrome extension's "Open app" button). It brings
// the window to the front.
func (a *App) handleUrlOpen(_ string) {
	if a.ctx == nil {
		return
	}
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// CheckForUpdates queries the registry immediately and reports the result.
func (a *App) CheckForUpdates() (*update.Update, error) {
	u, err := update.Check()
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateNow downloads, verifies and installs the given version, then restarts.
func (a *App) UpdateNow(version string) error {
	return update.Apply(version)
}

// checkForUpdatesFromMenu handles the native "Check for Updates…" menu item.
func (a *App) checkForUpdatesFromMenu() {
	u, err := update.Check()
	if err != nil {
		runtime.EventsEmit(a.ctx, "update-error", err.Error())
		return
	}
	if u.Available {
		runtime.EventsEmit(a.ctx, "update-available", &u)
	} else {
		runtime.EventsEmit(a.ctx, "update-up-to-date", &u)
	}
}

// ResponseResult is the outcome of an HTTP request, returned to the frontend.
type ResponseResult struct {
	Status      int               `json:"status"`
	StatusText  string            `json:"statusText"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	DurationMs  int64             `json:"durationMs"`
	ContentType string            `json:"contentType"`
	Error       string            `json:"error,omitempty"`
	Cancelled   bool              `json:"cancelled,omitempty"`
	DNSMs       int64             `json:"dnsMs,omitempty"`
	ConnectMs   int64             `json:"connectMs,omitempty"`
	TLSMs       int64             `json:"tlsMs,omitempty"`
	WaitMs      int64             `json:"waitMs,omitempty"`
	DownloadMs  int64             `json:"downloadMs,omitempty"`
}

// SendRequest performs an HTTP request and returns the full result. It is used
// by the manual request builder.
func (a *App) SendRequest(method, url string, headers map[string]string, body string) *ResponseResult {
	return a.do(method, url, headers, body)
}

// Fetch performs a GET request; used to follow related and pagination links.
// The headers of the request being followed are carried over (e.g. auth).
func (a *App) Fetch(url string, headers map[string]string) *ResponseResult {
	return a.do(http.MethodGet, url, headers, "")
}

// CancelRequest cancels the in-flight HTTP request, if any.
func (a *App) CancelRequest() {
	a.mu.Lock()
	cancel := a.cancelReq
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) do(method, url string, headers map[string]string, body string) *ResponseResult {
	res := &ResponseResult{}
	start := time.Now()

	// Phase timestamps for the "Тайминги" tab, captured via httptrace. Each
	// pair is zero until the corresponding callback fires, so diffMs below
	// turns "didn't happen" (e.g. no TLS on plain HTTP) into 0.
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
	a.mu.Lock()
	a.cancelReq = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.cancelReq = nil
		a.mu.Unlock()
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

	// "Ожидание" is measured from the moment the connection is ready (after
	// TLS, when there is one) to the first response byte.
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

// diffMs returns the elapsed milliseconds between two timestamps, or 0 when
// either side is missing (the phase never happened).
func diffMs(start, end time.Time) int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return end.Sub(start).Milliseconds()
}

// Analyze inspects a response body, reporting whether it is JSON:API and, if
// so, building the object graph used by the map view.
func (a *App) Analyze(body string) *jsonapi.Analysis {
	return jsonapi.Analyze([]byte(body))
}

// BridgePort returns the loopback port the browser extension connects to, so
// the empty state can name it without hardcoding the number in the template.
func (a *App) BridgePort() int {
	return bridge.DefaultPort
}

// PauseCapture tells the extension to stop capturing every tab. It is the
// "Приостановить перехват" action in the browser view; the extension stops its
// interceptors and replies with a fresh capture-state message.
func (a *App) PauseCapture() {
	if a.bridge != nil {
		a.bridge.Broadcast([]byte(`{"type":"pause"}`))
	}
}

// onCapturedRequest is the bridge handler: it forwards browser-captured
// requests to the frontend as a Wails event.
func (a *App) onCapturedRequest(req bridge.CapturedRequest) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "captured-request", req)
}

// onCaptureState forwards the extension's live capture status to the status bar.
func (a *App) onCaptureState(s bridge.CaptureState) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "capture-state", s)
}

// onCaptureDisconnected fires when the extension's socket drops, so the status
// bar stops claiming the extension is still connected.
func (a *App) onCaptureDisconnected() {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "capture-disconnected")
}
