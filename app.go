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
	// Guards cancelReq, mainWin and the parked events below.
	mu        sync.Mutex
	cancelReq context.CancelFunc

	bridge *bridge.Server

	app     *application.App
	mainWin *application.WebviewWindow

	ready atomic.Bool
	// Events raised before the window can take them: the update check runs at startup and a
	// deep link can arrive before the frontend has mounted, and both describe state the
	// window has to be told about anyway. The latest of each wins — an earlier tab doesn't
	// need reopening — and markReady plays them back.
	pendingTab    int
	pendingUpdate *update.Info
	// A check requested for the About window before it existed; taken by TakeUpdateCheckRequest.
	pendingUpdateCheck bool
}

func (a *App) setBridge(s *bridge.Server) {
	a.bridge = s
}

func (a *App) setApp(app *application.App) {
	a.app = app
}

func (a *App) setMainWindow(w *application.WebviewWindow) {
	a.mu.Lock()
	a.mainWin = w
	a.mu.Unlock()
}

func NewApp() *App {
	return &App{}
}

func (a *App) mainWindow() *application.WebviewWindow {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.mainWin
}

func (a *App) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	a.startUpdateCheck()
	return nil
}

// Ready is stored under the same lock the parking decision takes, and the swap happens with
// it, so a parked event can neither be missed by the flush nor slip in front of it.
func (a *App) markReady() {
	a.mu.Lock()
	a.ready.Store(true)
	tab, u := a.pendingTab, a.pendingUpdate
	a.pendingTab, a.pendingUpdate = 0, nil
	a.mu.Unlock()

	// The update first: the status bar takes its "доступна версия" link, then the rail
	// switches to the tab.
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

// openTab switches the rail to the extension's tab, parking the request if the window can't
// take events yet. The latest request wins: an earlier tab doesn't need reopening.
func (a *App) openTab(tab int) {
	a.mu.Lock()
	if !a.ready.Load() {
		a.pendingTab = tab
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	a.emit("open-tab", tab)
}

// announceUpdate tells the window a release is available, parking it the same way openTab does.
func (a *App) announceUpdate(u *update.Info) {
	a.mu.Lock()
	if !a.ready.Load() {
		a.pendingUpdate = u
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	a.emit("update-available", u)
}

func (a *App) focusMain() {
	if win := a.mainWindow(); win != nil {
		bringToFront(win)
	}
}

func (a *App) startUpdateCheck() {
	go func() {
		if u := update.StartupCheck(); u != nil {
			a.announceUpdate(u)
		}
	}()
}

func (a *App) Version() string {
	return update.CurrentVersion
}

// Build is the CI run number, empty for local builds.
func (a *App) Build() string {
	return update.CurrentBuild
}

func (a *App) Name() string {
	return appName
}

func (a *App) ToggleMaximize() {
	if win := a.mainWindow(); win != nil {
		win.ToggleMaximise()
	}
}

func (a *App) handleURLOpen(rawURL string) {
	a.focusMain()
	if tabID, ok := tabFromURL(rawURL); ok {
		a.openTab(tabID)
	}
}

func (a *App) onSecondInstance(data application.SecondInstanceData) {
	for _, arg := range data.Args {
		if strings.HasPrefix(arg, deepLinkScheme+"://") {
			a.handleURLOpen(arg)
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

// The update UI lives in the About window (design section 07), so these three are what the
// main window's check item and the native menu use to hand the work over to it.

// RequestUpdateCheck reuses or opens the About window and asks it to run a check. The request
// is parked as well as sent, because only the About window listens for it here — and a window
// that is being created right now has no listeners yet. It picks the parked request up on mount.
//
// The flag is cleared by TakeUpdateCheckRequest alone, never here: that is what makes the two
// paths deliver exactly one check between them, however they interleave.
func (a *App) RequestUpdateCheck() {
	a.mu.Lock()
	a.pendingUpdateCheck = true
	a.mu.Unlock()

	// Events go to every window in this version of Wails; nothing but the About window listens.
	if a.app != nil {
		a.app.Event.Emit("update-check")
	}
	a.ShowAbout()
}

// TakeUpdateCheckRequest reports and clears a request parked by RequestUpdateCheck. Destructive
// on purpose: it is called from both the mount and the event handler, and only the first caller
// may act on it, or an old request would fire a check the next time the window opens.
func (a *App) TakeUpdateCheckRequest() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	pending := a.pendingUpdateCheck
	a.pendingUpdateCheck = false
	return pending
}

// CheckForUpdates answers the About window's own check. A release it finds is also announced to
// the main window, so the status bar's link appears without waiting for the next launch — the
// one place a manual check and the startup check have to agree.
func (a *App) CheckForUpdates() (*update.Info, error) {
	u, err := update.Check()
	if err != nil {
		return nil, err
	}
	if u.Available {
		a.announceUpdate(&u)
	}
	return &u, nil
}

// UpdateStatus is what the last check saw, without touching the network.
func (a *App) UpdateStatus() (*update.Info, error) {
	s, err := update.Status()
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (a *App) UpdateNow(version string) error {
	return update.Install(version)
}

// Response is what a request comes back as: the body plus the per-phase timings.
type Response struct {
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

func (a *App) SendRequest(method, url string, headers map[string]string, body string) *Response {
	return a.sendTimed(method, url, headers, body)
}

func (a *App) Fetch(url string, headers map[string]string) *Response {
	return a.sendTimed(http.MethodGet, url, headers, "")
}

func (a *App) CancelRequest() {
	a.mu.Lock()
	cancel := a.cancelReq
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// The timed round trip behind SendRequest and Fetch: the client trace is what fills in the
// per-phase timings the response viewer draws.
func (a *App) sendTimed(method, url string, headers map[string]string, body string) *Response {
	res := &Response{}
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
		a.openTab(req.Tab)
	}
}
