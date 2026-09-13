package domain

// Setting keys, declared here so the store and the screen cannot disagree about their spelling.
const (
	SettingTheme            = "theme"
	SettingInspectorOpen    = "ui.inspectorOpen"
	SettingInspectorWidth   = "ui.inspectorWidth"
	SettingSideWidth        = "ui.sideWidth"
	SettingHistoryRetention = "history.retention"
)

// Theme is the user's choice; which palette it means is decided in the window, because "system" is
// a question only the window can ask.
type Theme string

const (
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
	ThemeSystem Theme = "system"
)

// Valid reports whether a stored or supplied theme is one the app knows.
func (t Theme) Valid() bool {
	return t == ThemeLight || t == ThemeDark || t == ThemeSystem
}

// Retention is how long history is kept. The count cap that has always applied stays: this is what
// removes records by age on top of it.
type Retention string

const (
	RetainWeek    Retention = "7"
	RetainMonth   Retention = "30"
	RetainForever Retention = "forever"
)

func (r Retention) Valid() bool {
	return r == RetainWeek || r == RetainMonth || r == RetainForever
}

// Settings is everything the app remembers about how it is set up, in the order the screen shows
// it. Defaults live in one place — the use case's — so a fresh database and a missing row agree.
type Settings struct {
	Theme            Theme     `json:"theme"`
	InspectorOpen    bool      `json:"inspectorOpen"`
	InspectorWidth   int       `json:"inspectorWidth"`
	SideWidth        int       `json:"sideWidth"`
	HistoryRetention Retention `json:"historyRetention"`
}

// DefaultSettings is what the app runs with before anyone has changed anything: the theme follows
// the system, the inspector is closed at its design width, and history is kept as it always was.
func DefaultSettings() Settings {
	return Settings{
		Theme:            ThemeSystem,
		InspectorOpen:    false,
		InspectorWidth:   DefaultInspectorWidth,
		SideWidth:        DefaultSideWidth,
		HistoryRetention: RetainForever,
	}
}

// Panel widths in pixels. The design names 288 for the list and 300 for the inspector; these are
// the values a window that has never been resized uses.
const (
	DefaultInspectorWidth = 300
	DefaultSideWidth      = 288
	// Widths are clamped on the way in: a window narrower than the sidebar plus a panel is not a
	// layout, and a stored value can come from a build with different limits.
	MinPanelWidth = 220
	MaxPanelWidth = 720
)
