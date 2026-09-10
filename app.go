package main

import (
	"context"
	"io"
	"net/http"
	"strings"
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

func (a *App) do(method, url string, headers map[string]string, body string) *ResponseResult {
	res := &ResponseResult{}
	start := time.Now()

	req, err := http.NewRequest(method, url, strings.NewReader(body))
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
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	res.DurationMs = time.Since(start).Milliseconds()
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

// Analyze inspects a response body, reporting whether it is JSON:API and, if
// so, building the object graph used by the map view.
func (a *App) Analyze(body string) *jsonapi.Analysis {
	return jsonapi.Analyze([]byte(body))
}

// onCapturedRequest is the bridge handler: it forwards browser-captured
// requests to the frontend as a Wails event.
func (a *App) onCapturedRequest(req bridge.CapturedRequest) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "captured-request", req)
}
