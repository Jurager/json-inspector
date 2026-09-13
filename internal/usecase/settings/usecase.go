// Package settings owns the app's preferences: what they are called, what they default to, and
// which of them the window has to be told about.
package settings

import (
	"context"
	"fmt"
	"strconv"

	"json-inspector/internal/domain"
)

// TopicThemeChanged is published whenever the theme moves. The About window listens too, which is
// why it is a broadcast and not an answer to whoever asked.
const TopicThemeChanged = "settings:theme"

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
	if open, ok := parseBool(stored[domain.SettingInspectorOpen]); ok {
		out.InspectorOpen = open
	}
	out.InspectorWidth = width(stored[domain.SettingInspectorWidth], out.InspectorWidth)
	out.SideWidth = width(stored[domain.SettingSideWidth], out.SideWidth)
	if retention := domain.Retention(stored[domain.SettingHistoryRetention]); retention.Valid() {
		out.HistoryRetention = retention
	}
	return out, nil
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
		return domain.Settings{}, fmt.Errorf("тема %q: %w", theme, domain.ErrNotAllowed)
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

// LayoutPatch is a partial update of the panel geometry: a nil field is left as it is.
type LayoutPatch struct {
	InspectorOpen  *bool `json:"inspectorOpen,omitempty"`
	InspectorWidth *int  `json:"inspectorWidth,omitempty"`
	SideWidth      *int  `json:"sideWidth,omitempty"`
}

// SetLayout remembers where the user put the panels. This is written often — every drag — so it
// saves exactly the fields it is given and nothing else.
func (u *UseCase) SetLayout(ctx context.Context, patch LayoutPatch) (domain.Settings, error) {
	if patch.InspectorOpen != nil {
		if err := u.save(ctx, domain.SettingInspectorOpen, strconv.FormatBool(*patch.InspectorOpen)); err != nil {
			return domain.Settings{}, err
		}
	}
	if patch.InspectorWidth != nil {
		if err := u.save(ctx, domain.SettingInspectorWidth, strconv.Itoa(clamp(*patch.InspectorWidth))); err != nil {
			return domain.Settings{}, err
		}
	}
	if patch.SideWidth != nil {
		if err := u.save(ctx, domain.SettingSideWidth, strconv.Itoa(clamp(*patch.SideWidth))); err != nil {
			return domain.Settings{}, err
		}
	}
	return u.Snapshot(ctx)
}

func (u *UseCase) SetRetention(ctx context.Context, retention domain.Retention) (domain.Settings, error) {
	if !retention.Valid() {
		return domain.Settings{}, fmt.Errorf("хранение истории %q: %w", retention, domain.ErrNotAllowed)
	}
	if err := u.save(ctx, domain.SettingHistoryRetention, string(retention)); err != nil {
		return domain.Settings{}, err
	}
	return u.Snapshot(ctx)
}

func (u *UseCase) save(ctx context.Context, key, value string) error {
	return u.store.SaveSetting(ctx, key, value)
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
