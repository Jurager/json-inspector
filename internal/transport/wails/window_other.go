//go:build !windows

package wails

import "github.com/wailsapp/wails/v3/pkg/application"

// glassShows reports whether this machine draws a material behind the window. macOS has one —
// vibrancy — but the webview above it only becomes transparent when the build carries the
// `private_mac_apis` tag, which this project does not pass: without it the window would be
// translucent with an opaque page on top of it, which is the same as being opaque and one
// surprise more. Linux has no material to draw at all.
//
// Both therefore keep an opaque window, and the glass there is the CSS the chrome paints — the
// same fill and blur, without something real behind it to show through.
func glassShows() bool {
	return false
}

// systemIsDark says which way the system's own theme points, for the "follow the system" setting.
// Only the Windows window needs the answer: macOS is told the default appearance and follows the
// system by itself.
func systemIsDark() bool {
	return false
}

// retintWindow has nothing to do where the window has no material: macOS takes its appearance once,
// at creation, and has no runtime setter for it, and the other platforms do not draw glass at all.
func retintWindow(*application.WebviewWindow, bool) {}
