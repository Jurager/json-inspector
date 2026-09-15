package wails

import (
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
	"json-inspector/internal/infra/updater"
)

// Host is the app itself as the services see it: the application handle, the main window, and
// the events that reach the frontend. It exists because the graph is built before the app is,
// so everything that has to be looked up rather than held needs a place to be parked.

const (
	// Must match `protocols:` in build/config.yml, which is what registers the
	// scheme with the OS (macOS Info.plist, Windows installer registry).
	deepLinkScheme = "json-inspector"

	windowMain     = "main"
	windowAbout    = "about"
	windowSettings = "settings"

	// The About panel is sized to its content (design section 07): 336 wide, fixed. Both
	// heights are measured from About.vue — the design's stack plus the chrome above it, which
	// differs per platform. Re-measure if the layout changes.
	aboutWidth          = 336
	aboutHeightBarred   = 449 // our own 52px bar above the body
	aboutHeightInset    = 425 // macOS: the body's own top padding stands in for the title bar
	aboutTitleBarHeight = 50

	// The settings window is the size the handoff draws (section 09): 1160 x 700. Its own minimum is
	// the point below which the 208px category column and a setting's row stop fitting side by side.
	settingsWidth     = 1160
	settingsHeight    = 700
	settingsMinWidth  = 820
	settingsMinHeight = 560
)

// singleInstanceKey encrypts the handoff between instances and never leaves the
// machine; the [32]byte conversion makes any length other than 32 a compile error.
var singleInstanceKey = [32]byte([]byte("json-inspector-v3-local-key-0001"))

// Host is the app's window and event surface: everything that has to be looked up or parked
// per-window rather than held as a plain reference. Services take it when they need to reach the
// frontend; nothing outside this package touches *application.App directly.
type Host struct {
	// Guards the app handle, the main window and the parked events below.
	mu      sync.Mutex
	app     *application.App
	mainWin *application.WebviewWindow

	ready atomic.Bool
	// The app's own name, which is what a window is titled until its page says otherwise: the words a
	// window is titled with are in the catalogue, and only the page has the catalogue.
	appName string
	// The theme, as the window needs it before it exists: a query string on the window's URL, so the
	// first paint is already in the right palette. Kept in step by whoever changes it.
	theme atomic.Value
	// The language, for the same reason and by the same route: the first frame is already written.
	language atomic.Value
	// What the material behind the window has already been tinted with — the palette, as the platform
	// spells it — and whether it has been told at all. Both are read and written from the request that
	// saved the choice, and from startup.
	tinted            atomic.Bool
	appliedAppearance atomic.Value
	// Events raised before the window can take them: the update check runs at startup and a
	// deep link can arrive before the frontend has mounted, and both describe state the
	// window has to be told about anyway. The latest of each wins — an earlier tab doesn't
	// need reopening — and MarkReady plays them back.
	pendingTab    int
	pendingUpdate *updater.Info
	// A check requested for the About window before it existed; taken by TakeUpdateCheckRequest.
	pendingUpdateCheck bool
}

func NewHost() *Host {
	host := &Host{}
	host.theme.Store("")
	host.language.Store("")
	return host
}

// SetLanguage is the one writer of the language. Unlike the theme it has nothing to re-tint: the
// window that is already open redraws on the broadcast, and this is only what the next window's URL
// will carry. The choice is stored unresolved — "system" is a question for the webview.
func (h *Host) SetLanguage(language domain.Language) {
	h.language.Store(string(language))
}

// SetAppName is what a window is titled with before its page has drawn: Wails wants a title at
// creation, and the page replaces it with the catalogue's word as it mounts.
func (h *Host) SetAppName(name string) {
	h.mu.Lock()
	h.appName = name
	h.mu.Unlock()
}

func (h *Host) windowTitle() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.appName
}

// Attach is the one writer of the application handle: the app cannot be built before the graph
// that its services come from, and the services need to reach it once it exists.
func (h *Host) Attach(app *application.App) {
	h.mu.Lock()
	h.app = app
	h.mu.Unlock()
}

func (h *Host) App() *application.App {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.app
}

func (h *Host) SetMainWindow(w *application.WebviewWindow) {
	h.mu.Lock()
	h.mainWin = w
	h.mu.Unlock()
}

func (h *Host) MainWindow() *application.WebviewWindow {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.mainWin
}

// MarkReady plays back what was raised before the page could take it, and nothing after that is
// parked again. Ready is stored under the same lock the parking decision takes, and the swap
// happens with it, so a parked event can neither be missed by the flush nor slip in front of it.
func (h *Host) MarkReady() {
	h.mu.Lock()
	h.ready.Store(true)
	tab, u := h.pendingTab, h.pendingUpdate
	h.pendingTab, h.pendingUpdate = 0, nil
	h.mu.Unlock()

	// The update first: the status bar takes its "доступна версия" link, then the rail
	// switches to the tab.
	if u != nil {
		h.Emit(eventUpdateAvailable, u)
	}
	if tab > 0 {
		h.Emit(eventOpenTab, tab)
	}
}

// Emit sends to the main window, dropping the event while that window still cannot take one.
func (h *Host) Emit(name string, data ...any) {
	if !h.ready.Load() {
		return
	}
	if win := h.MainWindow(); win != nil {
		win.EmitEvent(name, data...)
	}
}

// Broadcast sends to every window: theme and toast belong to the window set, not to the main
// window alone.
func (h *Host) Broadcast(name string, data ...any) {
	if app := h.App(); app != nil {
		app.Event.Emit(name, data...)
	}
}
