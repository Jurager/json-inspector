// Package settings owns the app's preferences: what they are called, what they default to, and
// which of them the window has to be told about.
package settings

import (
	"context"
	"encoding/json"
	"strconv"

	"json-inspector/internal/domain"
)

// TopicThemeChanged is published whenever the theme moves. The About window listens too, which is
// why it is a broadcast and not an answer to whoever asked.
const TopicThemeChanged = "settings:theme"

// TopicLanguageChanged is the same news for the interface language: every open window redraws, and
// the one that made the choice has already done so.
const TopicLanguageChanged = "settings:language"

// TopicChanged carries a whole snapshot for every other preference: the settings window writes and
// the main window draws the same rows — a retention it shows on the browser card, a wrap the raw
// viewer obeys — and neither is reloaded when the other one writes.
//
// The snapshot travels rather than the one value that moved, because what a listener has to do with
// it is the same in every case: replace what it shows. Geometry is the exception and does not
// publish — a drag writes once it settles, and a snapshot arriving mid-drag would undo the panel
// the pointer is still holding.
const TopicChanged = "settings:changed"

type UseCase struct {
	store    Store
	notifier Notifier
}

func NewUseCase(store Store, notifier Notifier) *UseCase {
	return &UseCase{store: store, notifier: notifier}
}

// Snapshot reads every preference, falling back to the default for anything absent or unreadable.
// A stored value from a newer build, or a hand-edited database, must not break the window.
func (u *UseCase) Snapshot(ctx context.Context) (domain.Settings, error) {
	stored, err := u.store.Settings(ctx)
	if err != nil {
		return domain.Settings{}, err
	}

	out := domain.DefaultSettings()
	if theme := domain.Theme(stored[domain.SettingTheme]); theme.Valid() {
		out.Theme = theme
	}
	if language := domain.Language(stored[domain.SettingLanguage]); language.Valid() {
		out.Language = language
	}
	if open, ok := parseBool(stored[domain.SettingInspectorOpen]); ok {
		out.InspectorOpen = open
	}
	out.InspectorWidth = width(stored[domain.SettingInspectorWidth], out.InspectorWidth)
	out.SideWidth = width(stored[domain.SettingSideWidth], out.SideWidth)
	if side := domain.ListSide(stored[domain.SettingListSide]); side.Valid() {
		out.ListSide = side
	}
	if retention := domain.Retention(stored[domain.SettingHistoryRetention]); retention.Valid() {
		out.HistoryRetention = retention
	}
	if filters, ok := parseCaptureFilters(stored[domain.SettingCaptureFilters]); ok {
		out.CaptureFilters = filters
	}
	if auto, ok := parseBool(stored[domain.SettingUpdateAuto]); ok {
		out.UpdateCheckAuto = auto
	}
	if channel := domain.UpdateChannel(stored[domain.SettingUpdateChannel]); channel.Valid() {
		out.UpdateChannel = channel
	}
	// Three whose absence means "on": a database written before they existed has no row for them, and
	// an editor that stopped wrapping lines because nobody had ever said so would be a change nobody
	// asked for. Only a value that parses and says false turns one off.
	if wrap, ok := parseBool(stored[domain.SettingWrapLines]); ok {
		out.WrapLines = wrap
	}
	if numbers, ok := parseBool(stored[domain.SettingLineNumbers]); ok {
		out.LineNumbers = numbers
	}
	if reopen, ok := parseBool(stored[domain.SettingReopenWorkspace]); ok {
		out.ReopenWorkspace = reopen
	}
	return out, nil
}

// announced hands a snapshot to every window and back to the caller: the shape of every setter
// whose row another window draws.
func (u *UseCase) announced(current domain.Settings, err error) (domain.Settings, error) {
	if err != nil {
		return current, err
	}
	u.notifier.Publish(TopicChanged, current)
	return current, nil
}

// Theme is the one preference the window needs before it exists: the first paint has to happen in
// the right palette, and asking Go over IPC is too late for that.
func (u *UseCase) Theme(ctx context.Context) (domain.Theme, error) {
	current, err := u.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	return current.Theme, nil
}

// SetTheme stores the choice and tells every window. The event carries the choice, not a resolved
// palette: "system" means "ask the platform", and only a window can.
func (u *UseCase) SetTheme(ctx context.Context, theme domain.Theme) (domain.Settings, error) {
	if !theme.Valid() {
		return domain.Settings{}, domain.Refuse(domain.CodeUnknownTheme, domain.ErrNotAllowed,
			domain.Args{"theme": string(theme)})
	}
	if err := u.save(ctx, domain.SettingTheme, string(theme)); err != nil {
		return domain.Settings{}, err
	}
	u.notifier.Publish(TopicThemeChanged, ThemeChanged{Theme: theme})
	return u.Snapshot(ctx)
}

// ThemeChanged is what a window receives when the theme moves.
type ThemeChanged struct {
	Theme domain.Theme `json:"theme"`
}

// Language is the other preference the window needs before it exists, for the same reason the theme
// is: the first frame is already drawn in a language, and asking Go over IPC is too late for that.
func (u *UseCase) Language(ctx context.Context) (domain.Language, error) {
	current, err := u.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	return current.Language, nil
}

// SetLanguage stores the choice and tells every window. Like the theme's, the event carries the
// choice and not a resolved language: "system" means "ask the webview", and only a window can.
func (u *UseCase) SetLanguage(
	ctx context.Context,
	language domain.Language,
) (domain.Settings, error) {
	if !language.Valid() {
		return domain.Settings{}, domain.Refuse(domain.CodeUnknownLanguage, domain.ErrNotAllowed,
			domain.Args{"language": string(language)})
	}
	if err := u.save(ctx, domain.SettingLanguage, string(language)); err != nil {
		return domain.Settings{}, err
	}
	u.notifier.Publish(TopicLanguageChanged, LanguageChanged{Language: language})
	return u.Snapshot(ctx)
}

// LanguageChanged is what a window receives when the interface language moves.
type LanguageChanged struct {
	Language domain.Language `json:"language"`
}

// LayoutPatch is a partial update of the panel geometry: a nil field is left as it is.
type LayoutPatch struct {
	InspectorOpen  *bool            `json:"inspectorOpen,omitempty"`
	InspectorWidth *int             `json:"inspectorWidth,omitempty"`
	SideWidth      *int             `json:"sideWidth,omitempty"`
	ListSide       *domain.ListSide `json:"listSide,omitempty"`
}

// SetLayout remembers where the user put the panels. This is written often — every drag — so it
// saves exactly the fields it is given and nothing else, and publishes only the one that is a
// choice
// rather than a measurement.
func (u *UseCase) SetLayout(ctx context.Context, patch LayoutPatch) (domain.Settings, error) {
	if patch.InspectorOpen != nil {
		if err := u.save(ctx, domain.SettingInspectorOpen,
			strconv.FormatBool(*patch.InspectorOpen)); err != nil {
			return domain.Settings{}, err
		}
	}
	if patch.InspectorWidth != nil {
		if err := u.save(ctx, domain.SettingInspectorWidth,
			strconv.Itoa(clamp(*patch.InspectorWidth))); err != nil {
			return domain.Settings{}, err
		}
	}
	if patch.SideWidth != nil {
		if err := u.save(ctx, domain.SettingSideWidth,
			strconv.Itoa(clamp(*patch.SideWidth))); err != nil {
			return domain.Settings{}, err
		}
	}
	// The side is an enum, not a measurement: there is nothing to clamp, and a value the window
	// cannot lay out is a bug worth naming rather than a preference worth keeping.
	if patch.ListSide != nil {
		if !patch.ListSide.Valid() {
			return domain.Settings{}, domain.Refuse(domain.CodeUnknownListSide, domain.ErrNotAllowed,
				domain.Args{"side": string(*patch.ListSide)})
		}
		if err := u.save(ctx, domain.SettingListSide, string(*patch.ListSide)); err != nil {
			return domain.Settings{}, err
		}
		// The side is moved by a click and not by a drag, and it is the one thing in this patch the
		// settings window draws: every window follows it, the way they follow a preference. Widths
		// and the inspector's flag stay unpublished — a panel under the pointer is nobody else's to
		// move, and the snapshot that followed a drag would fight the drag that is still happening.
		return u.announced(u.Snapshot(ctx))
	}
	return u.Snapshot(ctx)
}

func (u *UseCase) SetRetention(
	ctx context.Context,
	retention domain.Retention,
) (domain.Settings, error) {
	if !retention.Valid() {
		return domain.Settings{}, domain.Refuse(domain.CodeUnknownRetention, domain.ErrNotAllowed,
			domain.Args{"retention": string(retention)})
	}
	if err := u.save(ctx, domain.SettingHistoryRetention, string(retention)); err != nil {
		return domain.Settings{}, err
	}
	return u.announced(u.Snapshot(ctx))
}

// SetCaptureFilters stores the rules the extension applies. What they are is the app's to decide:
// the extension enforces them and shows them, and this is where they are kept.
func (u *UseCase) SetCaptureFilters(
	ctx context.Context,
	filters domain.CaptureFilters,
) (domain.Settings, error) {
	encoded, err := json.Marshal(filters)
	if err != nil {
		return domain.Settings{}, err
	}
	if err := u.save(ctx, domain.SettingCaptureFilters, string(encoded)); err != nil {
		return domain.Settings{}, err
	}
	return u.announced(u.Snapshot(ctx))
}

// SetUpdateCheck stores whether the app may look for a release on its own. Turning it off stops the
// background check only: a check the user asks for is theirs to ask, and an app that refuses one
// would be pretending the feature is gone rather than switched off.
func (u *UseCase) SetUpdateCheck(ctx context.Context, auto bool) (domain.Settings, error) {
	if err := u.save(ctx, domain.SettingUpdateAuto, strconv.FormatBool(auto)); err != nil {
		return domain.Settings{}, err
	}
	return u.announced(u.Snapshot(ctx))
}

// SetUpdateChannel stores which releases may be offered. The choice is not applied to a check that
// already ran: the next one asks the new channel, and the last answer stays on screen until then.
func (u *UseCase) SetUpdateChannel(
	ctx context.Context,
	channel domain.UpdateChannel,
) (domain.Settings, error) {
	if !channel.Valid() {
		return domain.Settings{}, domain.Refuse(domain.CodeUnknownChannel, domain.ErrNotAllowed,
			domain.Args{"channel": string(channel)})
	}
	if err := u.save(ctx, domain.SettingUpdateChannel, string(channel)); err != nil {
		return domain.Settings{}, err
	}
	return u.announced(u.Snapshot(ctx))
}

// EditorPatch is a partial update of what the raw viewer does with its lines: a nil field is left
// as it is, the same rule the layout patch follows.
type EditorPatch struct {
	WrapLines   *bool `json:"wrapLines,omitempty"`
	LineNumbers *bool `json:"lineNumbers,omitempty"`
}

// SetEditor remembers how the raw viewer draws a long line and its gutter. Both are the viewer's
// own business to apply — this only keeps the answer, and tells the window that is drawing one.
func (u *UseCase) SetEditor(ctx context.Context, patch EditorPatch) (domain.Settings, error) {
	if patch.WrapLines != nil {
		if err := u.save(ctx, domain.SettingWrapLines,
			strconv.FormatBool(*patch.WrapLines)); err != nil {
			return domain.Settings{}, err
		}
	}
	if patch.LineNumbers != nil {
		if err := u.save(ctx, domain.SettingLineNumbers,
			strconv.FormatBool(*patch.LineNumbers)); err != nil {
			return domain.Settings{}, err
		}
	}
	return u.announced(u.Snapshot(ctx))
}

// SetReopenWorkspace stores whether the app comes back to the space it was left in. Nothing is
// written to the pointer here: the choice is read where the pointer is read, so turning it back on
// returns the user to the space they were last in rather than to whichever one was on screen when
// they turned it off.
func (u *UseCase) SetReopenWorkspace(ctx context.Context, reopen bool) (domain.Settings, error) {
	if err := u.save(ctx, domain.SettingReopenWorkspace, strconv.FormatBool(reopen)); err != nil {
		return domain.Settings{}, err
	}
	return u.announced(u.Snapshot(ctx))
}

func (u *UseCase) save(ctx context.Context, key, value string) error {
	return u.store.SaveSetting(ctx, key, value)
}

// parseCaptureFilters reads the rules the extension filters by. An unreadable value keeps the
// default — the same rule every other setting follows — because a filter nobody can parse is not a
// reason to stop capturing.
func parseCaptureFilters(raw string) (domain.CaptureFilters, bool) {
	if raw == "" {
		return domain.CaptureFilters{}, false
	}
	var filters domain.CaptureFilters
	if err := json.Unmarshal([]byte(raw), &filters); err != nil {
		return domain.CaptureFilters{}, false
	}
	if filters.Hosts == nil {
		filters.Hosts = []string{}
	}
	return filters, true
}

func parseBool(raw string) (bool, bool) {
	if raw == "" {
		return false, false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false
	}
	return value, true
}

func width(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return clamp(value)
}

func clamp(value int) int {
	if value < domain.MinPanelWidth {
		return domain.MinPanelWidth
	}
	if value > domain.MaxPanelWidth {
		return domain.MaxPanelWidth
	}
	return value
}
