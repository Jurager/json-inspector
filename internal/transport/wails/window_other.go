//go:build !windows && !darwin

package wails

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
)

// glassShows reports whether this machine draws a material behind the window. Linux has none: a
// compositor may blur what is behind a window, but nothing here asks it to, and a window showing
// the desktop through it would read as a mistake rather than as glass. The window stays opaque and
// the glass is the CSS the chrome paints — the same fill and blur, with nothing real behind it.
func glassShows() bool {
	return false
}

// systemIsDark says which way the system's own theme points, for the "follow the system" setting.
// Only the Windows window needs the answer: the material there has two palettes and has to be told
// which, while this platform draws no material at all.
func systemIsDark() bool {
	return false
}

// appearanceFor is the palette the material is tinted by. There is no material here, so there is
// nothing to resolve and the choice travels as it is.
func appearanceFor(theme domain.Theme) domain.Theme {
	return theme
}

// retintWindow has nothing to do where the window has no material.
func retintWindow(*application.WebviewWindow, domain.Theme) {}

// prepareGlassWindow has nothing to open where there is no material to show.
func prepareGlassWindow(*application.WebviewWindow, domain.Theme) {}
