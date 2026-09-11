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

// App is the Wails service backing the whole UI.
// Its exported methods are callable from the frontend through the generated bindings.
//
// In v3 a service is a singleton shared by every window (the main window today,
// the environment editor later), so anything mutable lives under mu.
type App struct {
	mu        sync.Mutex
	cancelReq context.CancelFunc

	// bridge is the loopback WS server the extension connects to; it lets the
	// app push control messages back (pause) in the opposite direction.
	bridge *bridge.Server

	// Set before app.Run(). Nothing here is a package-level global any more:
	// every event and window action names its target explicitly. The About
	// window is not cached — it is looked up by name, so a closed one is
	// recreated rather than resurrected from a stale pointer.
	app  *application.App
	main *application.WebviewWindow

	// ready flips on events.Common.WindowRuntimeReady, i.e. once the frontend
	// has actually subscribed. Anything emitted before that is dropped silently,
	// so the two payloads that would be lost for good are buffered instead: a
	// deep link (open-tab) and an available update (StartupCheck only runs once
	// a day, so dropping it means the user waits until tomorrow).
	ready         atomic.Bool
	pendingTab    int
	pendingUpdate *update.Update
}

// setBridge wires the bridge server so PauseCapture can reach the extension.
func (a *App) setBridge(s *bridge.Server) {
	a.bridge = s
}

// setApp hands the service the application handle it needs to open windows.
func (a *App) setApp(app *application.App) {
	a.app = app
}

// setMainWindow registers the primary window. Events are addressed to it rather
// than to the app, which keeps a future environment-editor window out of the
// captured-request stream.
func (a *App) setMainWindow(w *application.WebviewWindow) {
	a.mu.Lock()
	a.main = w
	a.mu.Unlock()
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// mainWindow returns the primary window, or nil if it does not exist yet.
func (a *App) mainWindow() *application.WebviewWindow {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.main
}

// ServiceStartup runs once at start-up. Returning an error here aborts the whole
// application, so nothing that can fail may run inline — the update check goes
// to a goroutine.
func (a *App) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	a.startUpdateCheck()
	return nil
}

// markReady is called on WindowRuntimeReady, the precise moment the frontend
// has subscribed. It flushes whatever arrived while we were still booting.
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

// emit sends a window-scoped event. It is a no-op until the frontend has
// subscribed, which is what keeps early bridge callbacks from vanishing into a
// window that isn't listening yet.
func (a *App) emit(name string, data ...any) {
	if !a.ready.Load() {
		return
	}
	if win := a.mainWindow(); win != nil {
		win.EmitEvent(name, data...)
	}
}

// emitOpenTab delivers a "jump to this browser tab" request, or parks it until
// the UI is ready to receive it.
func (a *App) emitOpenTab(tab int) {
	if !a.ready.Load() {
		a.mu.Lock()
		a.pendingTab = tab
		a.mu.Unlock()
		return
	}
	a.emit("open-tab", tab)
}

// focusMain brings the main window to the front. Showing is unconditional: a
// link this app can't interpret further is still a request to come forward.
func (a *App) focusMain() {
	if win := a.mainWindow(); win != nil {
		bringToFront(win)
	}
}

// startUpdateCheck runs the once-a-day background version check and tells the
// UI when a newer version is available.
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

// Version returns the running version.
func (a *App) Version() string {
	return update.Current
}

// ToggleMaximize toggles the window zoom (double-click on the title bar).
func (a *App) ToggleMaximize() {
	if win := a.mainWindow(); win != nil {
		win.ToggleMaximise()
	}
}

// handleUrlOpen is called when the app is opened via the json-inspector://
// custom URL scheme (e.g. the Chrome extension's "Open app" button). It brings
// the window to the front and, when the link names a browser tab
// (json-inspector://open?tab=42), asks the UI to jump to that tab's requests.
func (a *App) handleUrlOpen(rawURL string) {
	a.focusMain()
	if tabID, ok := tabFromURL(rawURL); ok {
		a.emitOpenTab(tabID)
	}
}

// onSecondInstance is how every launch except the first arrives on all three
// platforms: the OS protocol handoff, or the user starting the binary again
// while the app is already running. The URL is carried in the args, and
// ApplicationLaunchedWithUrl does NOT fire on this path — so both have to end
// up in handleUrlOpen or deep links would only work with the app closed.
func (a *App) onSecondInstance(data application.SecondInstanceData) {
	for _, arg := range data.Args {
		if strings.HasPrefix(arg, deepLinkScheme+"://") {
			a.handleUrlOpen(arg)
			return
		}
	}
	a.focusMain()
}

// tabFromURL pulls the tab id out of json-inspector://open?tab=42. A link
// without one — or with something unparsable — is still a valid "show the
// window", so this reports ok=false instead of an error.
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
		a.emit("update-error", err.Error())
		return
	}
	if u.Available {
		a.emit("update-available", &u)
	} else {
		a.emit("update-up-to-date", &u)
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

// BridgePort returns the loopback port the browser extension connects to, so
// the empty state can name it without hardcoding the number in the template.
func (a *App) BridgePort() int {
	return bridge.DefaultPort
}

// The secret methods below are the frontend's only way to the keychain. They
// return the error untouched so the UI can tell "no keychain on this platform"
// apart from "this variable has no stored value" (the latter is an empty string
// with a nil error, see secrets.Get).

// SecretSet stores one environment secret.
func (a *App) SecretSet(envID, name, value string) error {
	return secrets.Set(envID, name, value)
}

// SecretGet reads one environment secret; "" means nothing is stored.
func (a *App) SecretGet(envID, name string) (string, error) {
	return secrets.Get(envID, name)
}

// SecretDelete removes one environment secret.
func (a *App) SecretDelete(envID, name string) error {
	return secrets.Delete(envID, name)
}

// PauseCapture tells the extension to stop capturing every tab. It is the
// "Приостановить перехват" action in the browser view; the extension stops its
// interceptors and replies with a fresh capture-state message.
func (a *App) PauseCapture() {
	if a.bridge != nil {
		a.bridge.Broadcast([]byte(`{"type":"pause"}`))
	}
}

// ResumeCapture is the other half: the extension re-injects into the tabs it
// had been recording before the pause.
func (a *App) ResumeCapture() {
	if a.bridge != nil {
		a.bridge.Broadcast([]byte(`{"type":"resume"}`))
	}
}

// onCapturedRequest is the bridge handler: it forwards browser-captured
// requests to the frontend as a Wails event.
func (a *App) onCapturedRequest(req bridge.CapturedRequest) {
	a.emit("captured-request", req)
}

// onCaptureState forwards the extension's live capture status to the status bar.
func (a *App) onCaptureState(s bridge.CaptureState) {
	a.emit("capture-state", s)
}

// onCaptureDisconnected fires when the extension's socket drops, so the status
// bar stops claiming the extension is still connected.
func (a *App) onCaptureDisconnected() {
	a.emit("capture-disconnected")
}

// onFocusRequest brings the window forward for the extension's "open this
// request" action. The extension asks over the live socket, so this path works
// whether or not the OS protocol handoff does.
func (a *App) onFocusRequest(req bridge.FocusRequest) {
	a.focusMain()
	if req.Tab > 0 {
		a.emitOpenTab(req.Tab)
	}
}
