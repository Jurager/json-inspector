package main

import "github.com/wailsapp/wails/v3/pkg/application"

const (
	// Must match `protocols:` in build/config.yml, which is what registers the
	// scheme with the OS (macOS Info.plist, Windows installer registry).
	deepLinkScheme = "json-inspector"

	windowMain  = "main"
	windowAbout = "about"
)

// singleInstanceKey encrypts the handoff between instances and never leaves the
// machine; the [32]byte conversion makes any length other than 32 a compile error.
var singleInstanceKey = [32]byte([]byte("json-inspector-v3-local-key-0001"))

// Bound to the frontend and to the native About menu item.
func (a *App) ShowAbout() {
	if a.app == nil {
		return
	}
	// A closed window leaves the manager, so look it up by name — a cached
	// pointer would stack a second window instead of focusing the existing one.
	if w, ok := a.app.Window.GetByName(windowAbout); ok {
		bringToFront(w)
		return
	}

	a.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             windowAbout,
		Title:            "О программе",
		Width:            460,
		Height:           560,
		MinWidth:         460,
		MinHeight:        560,
		// This flag disables resizing; v3 inverted it from v2's Resizable.
		DisableResize:    true,
		Frameless:        useCustomTitlebar(),
		BackgroundColour: application.NewRGB(30, 30, 30),
		URL:              "/about.html",
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 50,
		},
	})
}

func bringToFront(w application.Window) {
	w.Show()
	w.UnMinimise()
	w.Focus()
}

// Windows and Linux have no hidden-inset title bar and Wails' native menu there
// ignores the app theme, so they go frameless; must match every Frameless option.
func useCustomTitlebar() bool {
	return application.System.IsPlatform(application.PlatformWindows) ||
		application.System.IsPlatform(application.PlatformLinux)
}
