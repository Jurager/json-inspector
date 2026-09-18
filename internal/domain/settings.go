package domain

// Setting keys, declared here so the store and the screen cannot disagree about their spelling.
const (
	SettingTheme            = "theme"
	SettingLanguage         = "language"
	SettingInspectorOpen    = "ui.inspectorOpen"
	SettingInspectorWidth   = "ui.inspectorWidth"
	SettingSideWidth        = "ui.sideWidth"
	SettingListSide         = "ui.listSide"
	SettingHistoryRetention = "history.retention"
	SettingUpdateAuto       = "update.checkAutomatically"
	SettingUpdateChannel    = "update.channel"
	SettingCaptureFilters   = "capture.filters"
	SettingWrapLines        = "editor.wrapLines"
	SettingLineNumbers      = "editor.lineNumbers"
	SettingReopenWorkspace  = "workspace.reopenLast"
)

// CaptureFilters are the rules the extension applies before a request reaches the list: which hosts
// are kept, and which kinds of traffic are not worth keeping at all. They live in the app and are
// handed to the extension, because the app is where they are set and where the list they are about
// is read — an extension that decided them alone would be a second place for the same answer.
type CaptureFilters struct {
	// Hosts are the ones kept. Empty means every host, which is the app's answer until somebody
	// says otherwise: a filter that silently drops traffic nobody asked it to drop is worse than no
	// filter.
	Hosts []string `json:"hosts"`
	// Static is js, css, images and fonts — what a page is drawn from rather than what it does.
	Static bool `json:"static"`
	// Analytics are the hosts that report what the page did somewhere else.
	Analytics bool `json:"analytics"`
	// JSON keeps only answers that are JSON, which is a pair of a request and its response: the
	// type is known when the answer arrives, and the pair is dropped there rather than earlier.
	JSON bool `json:"json"`
}

// DefaultCaptureFilters is what a fresh installation starts with: static files skipped, which is
// what the extension did before the rules were anybody's to set.
func DefaultCaptureFilters() CaptureFilters {
	return CaptureFilters{Hosts: []string{}, Static: true, Analytics: false, JSON: false}
}

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

// Language is the user's choice of interface language, in the same shape as Theme. Which language
// "system" means is decided in the window, because the webview is the only side that can ask for
// it.
type Language string

const (
	LanguageSystem Language = "system"
	LanguageRU     Language = "ru"
	LanguageEN     Language = "en"
)

// Valid reports whether a stored or supplied language is one the app has catalogues for.
func (l Language) Valid() bool {
	return l == LanguageSystem || l == LanguageRU || l == LanguageEN
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

// ListSide is where the list panel lives: the same panel, told to sit at either edge of the work
// area, or to stay out of it. Hiding is a choice of the user's rather than the absence of one — a
// window whose list is put away was arranged that way, and is not a window that never had a list.
type ListSide string

const (
	ListSideLeft   ListSide = "left"
	ListSideRight  ListSide = "right"
	ListSideHidden ListSide = "hidden"
)

// Valid reports whether a stored or supplied side is one the window can lay out.
func (s ListSide) Valid() bool {
	return s == ListSideLeft || s == ListSideRight || s == ListSideHidden
}

// Settings is everything the app remembers about how it is set up, grouped the way the screen shows
// it. Defaults live in one place — the use case's — so a fresh database and a missing row agree.
type Settings struct {
	Theme            Theme          `json:"theme"`
	Language         Language       `json:"language"`
	InspectorOpen    bool           `json:"inspectorOpen"`
	InspectorWidth   int            `json:"inspectorWidth"`
	SideWidth        int            `json:"sideWidth"`
	ListSide         ListSide       `json:"listSide"`
	HistoryRetention Retention      `json:"historyRetention"`
	CaptureFilters   CaptureFilters `json:"captureFilters"`
	UpdateCheckAuto  bool           `json:"updateCheckAuto"`
	UpdateChannel    UpdateChannel  `json:"updateChannel"`
	// What the raw viewer does with a long line and with its gutter, and whether the app comes back
	// to the space it was left in. Booleans whose default is "on", which is why the absence of a row
	// has to be read as true rather than as false.
	WrapLines       bool `json:"wrapLines"`
	LineNumbers     bool `json:"lineNumbers"`
	ReopenWorkspace bool `json:"reopenWorkspace"`
}

// DefaultSettings is what the app runs with before anyone has changed anything.
func DefaultSettings() Settings {
	return Settings{
		Theme:            ThemeSystem,
		Language:         LanguageSystem,
		InspectorOpen:    false,
		InspectorWidth:   DefaultInspectorWidth,
		SideWidth:        DefaultSideWidth,
		ListSide:         ListSideLeft,
		HistoryRetention: RetainForever,
		CaptureFilters:   DefaultCaptureFilters(),
		// Updating is on until someone turns it off: an app that never mentions a release is one
		// where a security fix goes unnoticed, and "off" is one click away in the settings.
		UpdateCheckAuto: true,
		UpdateChannel:   ChannelStable,
		// The viewer wraps and numbers its lines, and the window opens where it was left: all three
		// are what the app already did before they could be switched off.
		WrapLines:       true,
		LineNumbers:     true,
		ReopenWorkspace: true,
	}
}

// Panel widths in pixels. The design names 262 for the list and 300 for the inspector; these are
// the values a window that has never been resized uses.
const (
	DefaultInspectorWidth = 300
	DefaultSideWidth      = 262
	// Widths are clamped on the way in: a window narrower than the sidebar plus a panel is not a
	// layout, and a stored value can come from a build with different limits.
	MinPanelWidth = 220
	MaxPanelWidth = 720
)
