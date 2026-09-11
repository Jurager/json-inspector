package main

import "github.com/wailsapp/wails/v3/pkg/application"

const (
	// deepLinkScheme is declared in build/config.yml under `protocols:`. That is
	// what puts it in the macOS Info.plist and in the Windows installer's
	// registry entries — on v2 only the .app carried it, so the Chrome
	// extension's "Открыть приложение" button did nothing on Windows.
	deepLinkScheme = "json-inspector"

	// Window names. Every window is created through openWindow, so adding the
	// environment editor later is one more constant and one more entry here.
	windowMain  = "main"
	windowAbout = "about"
)

// singleInstanceKey encrypts the handoff between instances. It only has to be
// stable and exactly 32 bytes — it never leaves the machine. The conversion is
// length-checked at compile time.
var singleInstanceKey = [32]byte([]byte("json-inspector-v3-local-key-0001"))

// ShowAbout opens the About window, or focuses it if it is already open.
// Bound to the frontend and wired to the native menu item.
func (a *App) ShowAbout() {
	if a.app == nil {
		return
	}
	// A closed window is removed from the manager, so look the name up instead
	// of trusting a cached pointer: a second click has to focus the window that
	// is already there rather than stack another one.
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
		// Note the inversion: this flag disables resizing, where the old v2
		// option enabled it.
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

// bringToFront un-minimises a window, shows it and gives it focus.
func bringToFront(w application.Window) {
	w.Show()
	w.UnMinimise()
	w.Focus()
}

// useCustomTitlebar reports whether the app draws its own title bar and caption
// buttons. Windows and Linux have no equivalent of macOS's hidden-inset title
// bar, and the native menu Wails would draw there doesn't follow the app theme,
// so those platforms go frameless. Must match the value main.go passes as
// WebviewWindowOptions.Frameless.
func useCustomTitlebar() bool {
	return application.System.IsPlatform(application.PlatformWindows) ||
		application.System.IsPlatform(application.PlatformLinux)
}
