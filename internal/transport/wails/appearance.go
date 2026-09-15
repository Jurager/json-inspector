package wails

// The material behind a window, and the palette it is drawn in. Neither is something the page can
// reach — the platform owns the frame — so the choice is said twice: once on the URL, for the frame
// before the first paint, and once here, for the frame that is already on screen.

import (
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
)

// The material behind a window, and the palette it is drawn in. Neither is something the page
// can reach — the platform owns the frame — so the choice is said twice: once on the URL, for
// the frame before the first paint, and once here, for the frame that is already on screen.

// SetTheme is the one writer of the theme: the windows this process has yet to create read the
// choice from its URL, and the one already there is re-tinted by it — the material behind a window
// is not something the page can reach, so the palette has to be said twice.
//
// The tint is the platform's own: on Windows moving it repaints the window's frame and shadow as
// well, a little later than the call, and macOS takes the repaint on its main thread. That is why
// the frontend tells its window first: the repaint then lands while the palette is changing rather
// than after it.
func (h *Host) SetTheme(theme domain.Theme) {
	h.theme.Store(string(theme))
	h.tint(theme)
}

// SystemThemeChanged is the system's own switch, which matters only while the app follows it. The
// material is the platform's and the page cannot reach it, so this is the only side that can move
// it — the frontend wipes its palette when the webview tells it the same news.
func (h *Host) SystemThemeChanged() {
	if theme, _ := h.theme.Load().(string); theme != string(domain.ThemeSystem) {
		return
	}
	// The choice stays "system" and the platform resolves it — a material that can follow the system
	// itself is left to do so, one that cannot is moved here.
	h.tint(domain.ThemeSystem)
}

// tint moves the material behind the window that is already there to the palette a theme resolves
// to.
func (h *Host) tint(theme domain.Theme) {
	appearance := appearanceFor(theme)
	if h.tinted.Load() {
		if applied, _ := h.appliedAppearance.Load().(string); applied == string(appearance) {
			// A different choice that resolves to the palette already in force — «системная» under a dark
			// system while the app is dark — must not repaint the window: there is nothing to re-tint.
			return
		}
	}
	h.tinted.Store(true)
	h.appliedAppearance.Store(string(appearance))
	// Only the window that shows the material: the About window is opaque.
	if main := h.MainWindow(); main != nil {
		retintWindow(main, appearance)
	}
}

// windowQuery is what goes on a window's URL: the pre-paint script reads it before the first frame,
// which is a thing no IPC call can do — the palette, the language, and whether the window has a
// material behind it that its ground must not cover. Both windows load the same stylesheet, so the
// second one is how a single page can be the ground of one window and the glass of another.
func (h *Host) windowQuery(translucent bool) string {
	query := url.Values{}
	if theme, _ := h.theme.Load().(string); theme != "" {
		query.Set("theme", theme)
	}
	if language, _ := h.language.Load().(string); language != "" {
		query.Set("lang", language)
	}
	if translucent {
		query.Set("translucent", "1")
	}
	if len(query) == 0 {
		return ""
	}
	return "?" + query.Encode()
}

// glassBackgroundType is how a window shows what is behind it: translucent where the machine can
// draw a material there, and solid where it cannot. The two are not a matter of taste — a window
// that lets the desktop through on a platform with nothing to blur shows the desktop under the
// app's chrome, which reads as a bug rather than as glass.
func glassBackgroundType() application.BackgroundType {
	if glassShows() {
		return application.BackgroundTypeTranslucent
	}
	return application.BackgroundTypeSolid
}

// glassWindowsBackdrop is the material Windows draws behind the webview. Acrylic rather than Mica:
// Mica tints the desktop wallpaper, and the chrome here is meant to be glass over what is behind
// the window. It is read only when the window is translucent.
func glassWindowsBackdrop() application.BackdropType {
	return application.Acrylic
}

// glassMacBackdrop is the macOS side of the same material. The frosted one — the legacy vibrancy
// material, which Wails falls back to — is what this asked for until it was measured on macOS 26:
// there it frosts nothing, passing the desktop through so sharply that the chrome reads as a plain
// translucent sheet. The system's own glass is the one that blurs, and Wails asks for it first and
// falls back to the legacy material where it does not exist, so one value covers both. It takes
// effect in a build carrying `private_mac_apis`, which is what makes the webview transparent.
func glassMacBackdrop() application.MacBackdrop {
	return application.MacBackdropLiquidGlass
}

// windowTheme is the appearance the native material is tinted by. It is set once, when the window
// is created — a window cannot be born with a material tinted by a choice nobody has made yet, and
// the stored theme is read before the window exists, so the two agree at startup. A theme switched
// while the app runs is moved by tint instead.
func windowTheme(theme domain.Theme) application.Theme {
	switch theme {
	case domain.ThemeDark:
		return application.Dark
	case domain.ThemeLight:
		return application.Light
	}
	// Not SystemDefault: that value leaves the window's own attributes alone — a window that never
	// says which way it is dark gets the light ones, and the material behind it would be tinted
	// against the app. So "follow the system" is resolved here, once, at creation.
	if systemIsDark() {
		return application.Dark
	}
	return application.Light
}

// macWindowAppearance is the same choice for macOS, which spells appearances as names.
func macWindowAppearance(theme domain.Theme) application.MacAppearanceType {
	switch theme {
	case domain.ThemeDark:
		return application.NSAppearanceNameDarkAqua
	case domain.ThemeLight:
		return application.NSAppearanceNameAqua
	default:
		return application.DefaultAppearance
	}
}
