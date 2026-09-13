//go:build windows

package wails

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// glassShows reports whether this machine draws a material behind the window. Acrylic needs Windows
// 11 22621 or later: below that the platform has only a blur-behind with no tint of its own — a
// different material, not a degraded one — and a window showing the desktop through it would read as
// a mistake rather than as glass. There the window stays opaque and the chrome is the CSS material.
func glassShows() bool {
	return w32.SupportsBackdropTypes()
}

// systemIsDark says which way the system's own theme points, for the "follow the system" setting.
// The window's materials cannot be told "system": a window created without a theme keeps the default
// light attributes, which is how a dark app ends up with a light-tinted material behind it.
func systemIsDark() bool {
	return w32.IsCurrentlyDarkMode()
}

// retintWindow moves the material behind a window that already exists to the other palette. One
// call, not two: the backdrop re-reads the window's own dark attribute, and applying the material
// on top of that would repaint the window a second time.
func retintWindow(window *application.WebviewWindow, dark bool) {
	if window == nil {
		return
	}
	if hwnd := uintptr(window.NativeWindow()); hwnd != 0 {
		w32.SetTheme(hwnd, dark)
	}
}
