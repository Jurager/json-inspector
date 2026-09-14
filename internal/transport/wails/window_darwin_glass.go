//go:build darwin && private_mac_apis

package wails

// glassShows reports whether this machine draws a material behind the window. macOS has one — the
// window's vibrancy — and this build is what lets it be seen: Wails makes the webview transparent
// through a private WebKit property, and without the tag that call is a stub, leaving an opaque
// page over a translucent window.
//
// The property is private because there is no public one, which is a trade this project makes
// knowingly: it ships as a disk image and a zip, not through the App Store, where it would be
// barred.
func glassShows() bool {
	return true
}
