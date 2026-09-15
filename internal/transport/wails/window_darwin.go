//go:build darwin

package wails

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
)

// glassShows is the one question the build tag answers for this platform: it lives in
// window_darwin_glass.go under `private_mac_apis` and in window_darwin_opaque.go without it.

// systemIsDark says which way the system's own theme points: macOS reads this from its preferences
// rather than from a window, and the app's own appearance would be the wrong answer — the app pins
// that one to its theme. appearanceFor is what asks, for the window that follows the system.
func systemIsDark() bool {
	// The app is reached through the global because this function has no window to hang off, and it
	// can be asked before the app exists — the theme is read before the window that will show it.
	app := application.Get()
	if app == nil || app.Env == nil {
		return false
	}
	return app.Env.IsDarkMode()
}

// appearanceFor is the palette the material behind the window is chosen by. "Follow the system" is
// resolved here rather than left to AppKit, as macOS could do: the material is one of two, so the
// system's own answer has to be known when it is picked, and the window's appearance is what
// carries that answer to the re-tint when the system changes under a running app.
func appearanceFor(theme domain.Theme) domain.Theme {
	if theme != domain.ThemeSystem {
		return theme
	}
	if systemIsDark() {
		return domain.ThemeDark
	}
	return domain.ThemeLight
}

// prepareGlassWindow lets the material through. Wails gives a translucent window its vibrancy view
// but never clears the window's own opaque flag — only its "transparent" backdrop does that, and
// this project does not use it — and an opaque window is drawn as a panel, with the material
// flattened into it: the chrome then sits on the system's window colour instead of on the desktop.
// That is a gap in the window options, so the flag is cleared here; it goes to the main thread for
// the same reason the re-tint does.
func prepareGlassWindow(window *application.WebviewWindow, theme domain.Theme) {
	if window == nil || !glassShows() {
		return
	}
	// The stored choice is resolved here as the re-tint resolves it, so a window and a switch agree.
	appearance := appearanceFor(theme)
	application.InvokeAsync(func() {
		if nsWindow := window.NativeWindow(); nsWindow != nil {
			clearWindowOpaque(nsWindow)
			setWindowAppearance(nsWindow, appearance)
			setWindowMaterial(nsWindow, appearance)
		}
	})
}

// retintWindow moves the material behind a window that already exists to another palette. macOS
// takes a window's appearance once, when it is created — this version of Wails has no setter for
// it — so without this a theme switched while the app runs would leave the vibrancy tinted the
// other way until the next launch, with the page already in the new palette.
//
// The native handle is asked for on the main thread rather than here: a window closed in between
// would otherwise be a dangling pointer by the time the call lands.
func retintWindow(window *application.WebviewWindow, theme domain.Theme) {
	if window == nil {
		return
	}
	application.InvokeAsync(func() {
		if nsWindow := window.NativeWindow(); nsWindow != nil {
			setWindowAppearance(nsWindow, theme)
			// The material is chosen by the theme too, so a switch moves both halves at once.
			setWindowMaterial(nsWindow, theme)
		}
	})
}
