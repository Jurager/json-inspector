package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/bridge"
	"json-inspector/internal/secrets"
	"json-inspector/internal/update"
)

type App struct {
	mu        sync.Mutex
	cancelReq context.CancelFunc

	bridge *bridge.Server

	app  *application.App
	main *application.WebviewWindow

	ready         atomic.Bool
	pendingTab    int
	pendingUpdate *update.Update
}

func (a *App) setBridge(s *bridge.Server) {
	a.bridge = s
}

func (a *App) setApp(app *application.App) {
	a.app = app
}

func (a *App) setMainWindow(w *application.WebviewWindow) {
	a.mu.Lock()
	a.main = w
	a.mu.Unlock()
}

func NewApp() *App {
	return &App{}
}

func (a *App) mainWindow() *application.WebviewWindow {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.main
}

func (a *App) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	a.startUpdateCheck()
	return nil
}

func (a *App) markReady() {
	a.ready.Store(true)

	a.mu.Lock()
	tab, u := a.pendingTab, a.pendingUpdate
	a.pendingTab, a.pendingUpdate = 0, nil
	a.mu.Unlock()

	if u != nil {
		a.emit("update-available", u)
	}
	if tab > 0 {
		a.emit("open-tab", tab)
	}
}

func (a *App) emit(name string, data ...any) {
	if !a.ready.Load() {
		return
	}
	if win := a.mainWindow(); win != nil {
		win.EmitEvent(name, data...)
	}
}

func (a *App) emitOpenTab(tab int) {
	if !a.ready.Load() {
		a.mu.Lock()
		a.pendingTab = tab
		a.mu.Unlock()
		return
	}
	a.emit("open-tab", tab)
}

func (a *App) focusMain() {
	if win := a.mainWindow(); win != nil {
		bringToFront(win)
	}
}

func (a *App) startUpdateCheck() {
	go func() {
		u := update.StartupCheck()
		if u == nil {
			return
		}
		if !a.ready.Load() {
			a.mu.Lock()
			a.pendingUpdate = u
			a.mu.Unlock()
			return
		}
		a.emit("update-available", u)
	}()
}

func (a *App) Version() string {
	return update.Current
}

func (a *App) Name() string {
	return appName
}

func (a *App) ToggleMaximize() {
	if win := a.mainWindow(); win != nil {
		win.ToggleMaximise()
	}
}

func (a *App) handleUrlOpen(rawURL string) {
	a.focusMain()
	if tabID, ok := tabFromURL(rawURL); ok {
		a.emitOpenTab(tabID)
	}
}

func (a *App) onSecondInstance(data application.SecondInstanceData) {
	for _, arg := range data.Args {
		if strings.HasPrefix(arg, deepLinkScheme+"://") {
			a.handleUrlOpen(arg)
			return
		}
	}
	a.focusMain()
}

func tabFromURL(rawURL string) (int, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, false
	}
	value := u.Query().Get("tab")
	if value == "" {
		return 0, false
	}
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func (a *App) CheckForUpdates() (*update.Update, error) {
	u, err := update.Check()
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (a *App) UpdateNow(version string) error {
	return update.Apply(version)
}

func (a *App) checkForUpdatesFromMenu() {
	u, err := update.Check()
	if err != nil {
		a.emit("update-error", err.Error())
		return
	}
	if u.Available {
		a.emit("update-available", &u)
	} else {
		a.emit("update-up-to-date", &u)
	}
}

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

func (a *App) SendRequest(method, url string, headers map[string]string, body string) *ResponseResult {
	return a.do(method, url, headers, body)
}

func (a *App) Fetch(url string, headers map[string]string) *ResponseResult {
	return a.do(http.MethodGet, url, headers, "")
}

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

func diffMs(start, end time.Time) int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return end.Sub(start).Milliseconds()
}

func (a *App) BridgePort() int {
	return bridge.DefaultPort
}

func (a *App) SecretSet(envID, name, value string) error {
	return secrets.Set(envID, name, value)
}

func (a *App) SecretGet(envID, name string) (string, error) {
	return secrets.Get(envID, name)
}

func (a *App) SecretDelete(envID, name string) error {
	return secrets.Delete(envID, name)
}

func (a *App) PauseCapture() {
	if a.bridge != nil {
		a.bridge.Broadcast([]byte(`{"type":"pause"}`))
	}
}

func (a *App) ResumeCapture() {
	if a.bridge != nil {
		a.bridge.Broadcast([]byte(`{"type":"resume"}`))
	}
}

func (a *App) onCapturedRequest(req bridge.CapturedRequest) {
	a.emit("captured-request", req)
}

func (a *App) onCaptureState(s bridge.CaptureState) {
	a.emit("capture-state", s)
}

func (a *App) onCaptureDisconnected() {
	a.emit("capture-disconnected")
}

func (a *App) onFocusRequest(req bridge.FocusRequest) {
	a.focusMain()
	if req.Tab > 0 {
		a.emitOpenTab(req.Tab)
	}
}
