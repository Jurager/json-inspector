package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/settings"
)

// SettingsService is the preferences the window reads at startup and writes as the user works.
type SettingsService struct {
	settings *settings.UseCase
	host     *Host
}

func NewSettingsService(uc *settings.UseCase, host *Host) *SettingsService {
	return &SettingsService{settings: uc, host: host}
}

func (s *SettingsService) Snapshot(ctx context.Context) (domain.Settings, error) {
	return s.settings.Snapshot(ctx)
}

// SetTheme stores the choice and broadcasts it, so every window follows — the About window is not
// told by whoever changed it.
func (s *SettingsService) SetTheme(
	ctx context.Context,
	theme domain.Theme,
) (domain.Settings, error) {
	saved, err := s.settings.SetTheme(ctx, theme)
	if err != nil {
		return saved, err
	}
	// The frontend follows the broadcast; the window's own material has to be told separately, because
	// it lives outside the page.
	s.host.SetTheme(theme)
	return saved, nil
}

// SetLanguage stores the choice and broadcasts it, the same way the theme is broadcast — and for
// the same reason: no window is the one that tells the others.
func (s *SettingsService) SetLanguage(
	ctx context.Context,
	language domain.Language,
) (domain.Settings, error) {
	saved, err := s.settings.SetLanguage(ctx, language)
	if err != nil {
		return saved, err
	}
	// The page redraws on the broadcast; the window's URL is what the *next* window will read, so Go
	// keeps the choice as well.
	s.host.SetLanguage(language)
	return saved, nil
}

func (s *SettingsService) SetLayout(
	ctx context.Context,
	patch settings.LayoutPatch,
) (domain.Settings, error) {
	return s.settings.SetLayout(ctx, patch)
}

// SetEditor stores what the raw viewer does with a long line and with its gutter. The viewer reads
// it from the broadcast, so the window that changed it does not have to hand it over.
func (s *SettingsService) SetEditor(
	ctx context.Context,
	patch settings.EditorPatch,
) (domain.Settings, error) {
	return s.settings.SetEditor(ctx, patch)
}

// SetReopenWorkspace stores whether the app comes back to the space it was left in. It takes effect
// at the next launch, which is where the pointer is resolved.
func (s *SettingsService) SetReopenWorkspace(
	ctx context.Context,
	reopen bool,
) (domain.Settings, error) {
	return s.settings.SetReopenWorkspace(ctx, reopen)
}

// SetCaptureFilters stores the rules the extension filters by. It does not hand them over: the
// bridge is a different service, and the window calls it — one place to fail is not two.
func (s *SettingsService) SetCaptureFilters(
	ctx context.Context,
	filters domain.CaptureFilters,
) (domain.Settings, error) {
	return s.settings.SetCaptureFilters(ctx, filters)
}

func (s *SettingsService) SetRetention(
	ctx context.Context,
	retention domain.Retention,
) (domain.Settings, error) {
	return s.settings.SetRetention(ctx, retention)
}

// SetUpdateCheck stores whether the app may look for a release on its own. The settings window
// turns it off, and the browser card draws how it is kept — which is why the snapshot it saves
// travels to the other windows.
func (s *SettingsService) SetUpdateCheck(
	ctx context.Context,
	auto bool,
) (domain.Settings, error) {
	return s.settings.SetUpdateCheck(ctx, auto)
}

// SetUpdateChannel stores which releases may be offered. A check already run keeps its answer on
// screen until the next one asks the new channel.
func (s *SettingsService) SetUpdateChannel(
	ctx context.Context,
	channel domain.UpdateChannel,
) (domain.Settings, error) {
	return s.settings.SetUpdateChannel(ctx, channel)
}
