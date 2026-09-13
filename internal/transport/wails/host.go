package wails

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
	"json-inspector/internal/infra/updater"
)

const (
	// Must match `protocols:` in build/config.yml, which is what registers the
	// scheme with the OS (macOS Info.plist, Windows installer registry).
	deepLinkScheme = "json-inspector"

	windowMain  = "main"
	windowAbout = "about"

	// The About panel is sized to its content (design section 07): 336 wide, fixed. Both
	// heights are measured from About.vue — the design's stack plus the chrome above it, which
	// differs per platform. Re-measure if the layout changes.
	aboutWidth          = 336
	aboutHeightBarred   = 449 // our own 52px bar above the body
	aboutHeightInset    = 425 // macOS: the body's own top padding stands in for the title bar
	aboutTitleBarHeight = 50
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
	// The theme, as the window needs it before it exists: a query string on the window's URL, so the
	// first paint is already in the right palette. Kept in step by whoever changes it.
	theme atomic.Value
	// What the window has already been tinted with: which way it is dark, and whether it has been told
	// at all. Both are read and written from the request that saved the choice, and from startup.
	tinted      atomic.Bool
	appliedDark atomic.Bool
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
	return host
}

// SetTheme is the one writer of the theme: the windows this process has yet to create read the
// choice from its URL, and the one already there is re-tinted by it — the material behind a window
// is not something the page can reach, so the palette has to be said twice.
//
// The tint is Windows' own attribute, and moving it repaints the window's frame and shadow as well,
// a little later than the call. That is why the frontend tells its window first: the repaint then
// lands while the palette is changing rather than after it.
func (h *Host) SetTheme(theme domain.Theme) {
	h.theme.Store(string(theme))
	h.tint(theme == domain.ThemeDark || (theme == domain.ThemeSystem && systemIsDark()))
}

// SystemThemeChanged is the system's own switch, which matters only while the app follows it. The
// material is the platform's and the page cannot reach it, so this is the only side that can move it
// — the frontend wipes its palette when the webview tells it the same news.
func (h *Host) SystemThemeChanged() {
	if theme, _ := h.theme.Load().(string); theme != string(domain.ThemeSystem) {
		return
	}
	h.tint(systemIsDark())
}

// tint moves the material behind the window that is already there to the palette a theme resolves to.
func (h *Host) tint(dark bool) {
	if h.tinted.Load() && dark == h.appliedDark.Load() {
		// A different choice that resolves to the palette already in force — «системная» under a dark
		// system while the app is dark — must not repaint the window: there is nothing to re-tint.
		return
	}
	h.tinted.Store(true)
	h.appliedDark.Store(dark)
	// Only the window that shows the material: the About window is opaque.
	if main := h.MainWindow(); main != nil {
		retintWindow(main, dark)
	}
}

// windowQuery is what goes on a window's URL: the pre-paint script reads it before the first frame,
// which is a thing no IPC call can do — the palette, and whether the window has a material behind it
// that its ground must not cover. Both windows load the same stylesheet, so the second one is how a
// single page can be the ground of one window and the glass of another.
func (h *Host) windowQuery(translucent bool) string {
	query := url.Values{}
	if theme, _ := h.theme.Load().(string); theme != "" {
		query.Set("theme", theme)
	}
	if translucent {
		query.Set("translucent", "1")
	}
	if len(query) == 0 {
		return ""
	}
	return "?" + query.Encode()
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

// Ready is stored under the same lock the parking decision takes, and the swap happens with
// it, so a parked event can neither be missed by the flush nor slip in front of it.
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

// OpenTab switches the rail to the extension's tab, parking the request if the window can't
// take events yet. The latest request wins: an earlier tab doesn't need reopening.
func (h *Host) OpenTab(tab int) {
	h.mu.Lock()
	if !h.ready.Load() {
		h.pendingTab = tab
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.Emit(eventOpenTab, tab)
}

// AnnounceUpdate tells the window a release is available, parking it the same way OpenTab does.
func (h *Host) AnnounceUpdate(u *updater.Info) {
	h.mu.Lock()
	if !h.ready.Load() {
		h.pendingUpdate = u
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.Emit(eventUpdateAvailable, u)
}

// RequestUpdateCheck parks a check for the About window and asks it to run one. The request is
// parked as well as sent, because only the About window listens for it here — and a window that
// is being created right now has no listeners yet. It picks the parked request up on mount.
//
// The flag is cleared by TakeUpdateCheckRequest alone, never here: that is what makes the two
// paths deliver exactly one check between them, however they interleave.
func (h *Host) RequestUpdateCheck() {
	h.mu.Lock()
	h.pendingUpdateCheck = true
	h.mu.Unlock()

	// Events go to every window in this version of Wails; nothing but the About window listens.
	h.Broadcast(eventUpdateCheck)
	h.ShowAbout()
}

// TakeUpdateCheckRequest reports and clears a request parked by RequestUpdateCheck. Destructive
// on purpose: it is called from both the mount and the event handler, and only the first caller
// may act on it, or an old request would fire a check the next time the window opens.
func (h *Host) TakeUpdateCheckRequest() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	pending := h.pendingUpdateCheck
	h.pendingUpdateCheck = false
	return pending
}

func (h *Host) FocusMain() {
	if win := h.MainWindow(); win != nil {
		bringToFront(win)
	}
}

func (h *Host) ToggleMaximize() {
	if win := h.MainWindow(); win != nil {
		win.ToggleMaximise()
	}
}

// OpenFile asks the user for a file to read and answers with its path, or with nothing when the
// dialog was closed — closing a dialog is not a failure.
//
// The dialogs live on the Host because they are the desktop's, and a feature that needs a file asks
// for one here instead of knowing how this app talks to the system.
func (h *Host) OpenFile(title string, filters ...application.FileFilter) (string, error) {
	app := h.App()
	if app == nil {
		return "", errors.New("окно ещё не создано")
	}
	dialog := app.Dialog.OpenFile().SetTitle(title)
	for _, filter := range filters {
		dialog = dialog.AddFilter(filter.DisplayName, filter.Pattern)
	}
	path, err := dialog.PromptForSingleSelection()
	return path, withoutCancellation(err)
}

// withoutCancellation turns a closed dialog into "nothing happened". Wails answers a cancelled
// dialog with an error and not with an empty path, and the sentinel it uses lives in an internal
// package of the module — so the text is all there is to go by. Getting this wrong is not cosmetic:
// without it, closing a file dialog reports a failure the user did not have.
func withoutCancellation(err error) error {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "cancel") {
		return nil
	}
	return err
}

// SaveFile asks where to write a file, offering the name to start from and the filters the caller
// wants offered, and answers with the path the user chose — or with nothing, when they chose none.
func (h *Host) SaveFile(title string, suggestedName string, filters ...application.FileFilter) (string, error) {
	app := h.App()
	if app == nil {
		return "", errors.New("окно ещё не создано")
	}
	path, err := app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:   title,
		Message: title,
		// The name is a suggestion: the dialog opens on it, and the user types over it or picks
		// another place, which is what a save dialog is for.
		Filename: suggestedName,
		Filters:  filters,
	}).PromptForSingleSelection()
	return path, withoutCancellation(err)
}

func (h *Host) HandleURLOpen(rawURL string) {
	h.FocusMain()
	if tabID, ok := tabFromURL(rawURL); ok {
		h.OpenTab(tabID)
	}
}

func (h *Host) OnSecondInstance(data application.SecondInstanceData) {
	for _, arg := range data.Args {
		if strings.HasPrefix(arg, deepLinkScheme+"://") {
			h.HandleURLOpen(arg)
			return
		}
	}
	h.FocusMain()
}

// Bound to the frontend and to the native About menu item.
func (h *Host) ShowAbout() {
	app := h.App()
	if app == nil {
		return
	}
	// A closed window leaves the manager, so look it up by name — a cached
	// pointer would stack a second window instead of focusing the existing one.
	if w, ok := h.aboutWindow(); ok {
		bringToFront(w)
		return
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      windowAbout,
		Title:     "О программе",
		Width:     aboutWidth,
		Height:    aboutHeight(),
		MinWidth:  aboutWidth,
		MinHeight: aboutHeight(),
		// This flag disables resizing; v3 inverted it from v2's Resizable.
		DisableResize:    true,
		Frameless:        UseCustomTitlebar(),
		BackgroundColour: application.NewRGB(30, 30, 30),
		URL:              "/about.html" + h.windowQuery(false),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: aboutTitleBarHeight,
		},
	})
}

// aboutWindow is the open About window, if there is one — looked up by name every time, since
// the user can close it and a stored handle would then point at a destroyed window.
func (h *Host) aboutWindow() (application.Window, bool) {
	app := h.App()
	if app == nil {
		return nil, false
	}
	return app.Window.GetByName(windowAbout)
}

// Close stops the app the way the window's close button would.
func (h *Host) Quit() {
	if app := h.App(); app != nil {
		app.Quit()
	}
}

// Windows and Linux have no hidden-inset title bar and Wails' native menu there
// ignores the app theme, so they go frameless; must match every Frameless option.
func UseCustomTitlebar() bool {
	return application.System.IsPlatform(application.PlatformWindows) ||
		application.System.IsPlatform(application.PlatformLinux)
}

// glassBackgroundType is how a window shows what is behind it: translucent where the machine can
// draw a material there, and solid where it cannot. The two are not a matter of taste — a window
// that lets the desktop through on a platform with nothing to blur shows the desktop under the
// app's chrome, which reads as a bug rather than as glass.
func glassBackgroundType() application.BackgroundType {
	if glassShows() {
		return application.BackgroundTypeTranslucent
	}
	return application.BackgroundTypeSolid
}

// glassWindowsBackdrop is the material Windows draws behind the webview. Acrylic rather than Mica:
// Mica tints the desktop wallpaper, and the chrome here is meant to be glass over what is behind the
// window. It is read only when the window is translucent.
func glassWindowsBackdrop() application.BackdropType {
	return application.Acrylic
}

// glassMacBackdrop is the macOS side of the same material: the frosted one rather than Liquid Glass,
// which needs macOS 26 and would make the app look like a different product. It is inert until the
// build carries `private_mac_apis` — see glassShows.
func glassMacBackdrop() application.MacBackdrop {
	return application.MacBackdropTranslucent
}

// windowTheme is the appearance the native material is tinted by. It is set once, when the window is
// created — this version of Wails has no runtime setter — and the stored theme is read before the
// window exists, so the two agree at startup. A theme switched while the app runs reaches the
// material on the next launch; the CSS half of the material switches at once.
func windowTheme(theme domain.Theme) application.Theme {
	switch theme {
	case domain.ThemeDark:
		return application.Dark
	case domain.ThemeLight:
		return application.Light
	}
	// Not SystemDefault: that value leaves the window's own attributes alone — a window that never
	// says which way it is dark gets the light ones, and the material behind it would be tinted
	// against the app. So "follow the system" is resolved here, once, at creation.
	if systemIsDark() {
		return application.Dark
	}
	return application.Light
}

// macWindowAppearance is the same choice for macOS, which spells appearances as names.
func macWindowAppearance(theme domain.Theme) application.MacAppearanceType {
	switch theme {
	case domain.ThemeDark:
		return application.NSAppearanceNameDarkAqua
	case domain.ThemeLight:
		return application.NSAppearanceNameAqua
	default:
		return application.DefaultAppearance
	}
}

// aboutHeight is the panel's height on this platform.
func aboutHeight() int {
	if UseCustomTitlebar() {
		return aboutHeightBarred
	}
	return aboutHeightInset
}

func bringToFront(w application.Window) {
	w.Show()
	w.UnMinimise()
	w.Focus()
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
