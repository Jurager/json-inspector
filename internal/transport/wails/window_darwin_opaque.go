//go:build darwin && !private_mac_apis

package wails

// glassShows reports whether this machine draws a material behind the window. macOS has one, but
// only a build carrying the `private_mac_apis` tag can show it — see window_darwin_glass.go.
// Without the tag the window stays opaque: a translucent window with an opaque page on top of it is
// the same as being opaque, and one surprise more. The glass here is the CSS the chrome paints.
func glassShows() bool {
	return false
}
