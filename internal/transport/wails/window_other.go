//go:build !windows && !darwin

package wails

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
)

// glassShows is false: Linux has no material to show. A compositor may blur what is behind a
// window, but nothing here asks it to, and a window showing the desktop through would read as a
// mistake. The glass here is the CSS the chrome paints.
func glassShows() bool {
	return false
}

// systemIsDark is false here: only the Windows material has two palettes and has to be told which.
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
