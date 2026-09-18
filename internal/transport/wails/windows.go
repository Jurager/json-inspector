package wails

// The windows that are not the main one, and the geometry the design gives them.

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ShowAbout opens the About panel, or brings the open one forward. It is a window of its own rather
// than a system dialog, so the native menu item and the frontend both call it.
func (h *Host) ShowAbout() {
	app := h.App()
	if app == nil {
		return
	}
	// A closed window leaves the manager, so look it up by name — a cached
	// pointer would stack a second window instead of focusing the existing one.
	if w, ok := h.windowByName(windowAbout); ok {
		bringToFront(w)
		return
	}

	// The title is the app's own name until the page replaces it: the word on a title bar is the
	// page's to give, and "About" is not a name Go could write in a language it does not know.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      windowAbout,
		Title:     h.windowTitle(),
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

// ShowUpdate opens the window that describes the release the last check found, or brings the open
// one forward. It is a window of its own rather than a dialog inside the About window because what
// it holds does not fit there: the About panel is 336 wide and this one is 560.
func (h *Host) ShowUpdate() {
	app := h.App()
	if app == nil {
		return
	}
	if w, ok := h.windowByName(windowUpdate); ok {
		bringToFront(w)
		return
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      windowUpdate,
		Title:     h.windowTitle(),
		Width:     updateWidth,
		Height:    updateHeight,
		MinWidth:  updateWidth,
		MinHeight: updateHeight,
		// This flag disables resizing; v3 inverted it from v2's Resizable.
		DisableResize:    true,
		Frameless:        UseCustomTitlebar(),
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/update.html" + h.windowQuery(false),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: aboutTitleBarHeight,
		},
	})
}

func (h *Host) windowByName(name string) (application.Window, bool) {
	app := h.App()
	if app == nil {
		return nil, false
	}
	return app.Window.GetByName(name)
}

// ShowSettings opens the preferences window, or brings the open one forward.
//
// A category that is named is the one the window shows: the account menu sends people to the
// account, and the gear leaves whatever was there. A window that is not created yet reads the
// category off its address, an open one is told — a page already on screen reads no URL twice.
func (h *Host) ShowSettings(category string) {
	app := h.App()
	if app == nil {
		return
	}
	// A closed window leaves the manager, so this is a lookup by name and never a cached handle.
	if w, ok := h.windowByName(windowSettings); ok {
		bringToFront(w)
		if category != "" {
			w.EmitEvent(eventSettingsTab, category)
		}
		return
	}

	params := h.windowParams(false)
	if category != "" {
		params.Set("tab", category)
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: windowSettings,
		// The page replaces this with the title in the chosen language as it mounts: the words live in
		// the catalogue, and a window's own name is the one thing Go cannot know in that language.
		Title:            h.windowTitle(),
		Width:            settingsWidth,
		Height:           settingsHeight,
		MinWidth:         settingsMinWidth,
		MinHeight:        settingsMinHeight,
		Frameless:        UseCustomTitlebar(),
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/settings.html" + encodeQuery(params),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: aboutTitleBarHeight,
		},
	})
}

// UseCustomTitlebar reports whether this platform draws the window without a system title bar.
// Windows and Linux have no hidden-inset bar and Wails' native menu there ignores the app theme, so
// they go frameless; must match every Frameless option.
func UseCustomTitlebar() bool {
	return application.System.IsPlatform(application.PlatformWindows) ||
		application.System.IsPlatform(application.PlatformLinux)
}

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
