package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/settings"
)

// SettingsService is the preferences the window reads at startup and writes as the user works.
type SettingsService struct {
	settings *settings.UseCase
}

func NewSettingsService(uc *settings.UseCase) *SettingsService {
	return &SettingsService{settings: uc}
}

func (s *SettingsService) Snapshot(ctx context.Context) (domain.Settings, error) {
	return s.settings.Snapshot(ctx)
}

// SetTheme stores the choice and broadcasts it, so every window follows — the About window is not
// told by whoever changed it.
func (s *SettingsService) SetTheme(ctx context.Context, theme domain.Theme) (domain.Settings, error) {
	return s.settings.SetTheme(ctx, theme)
}

func (s *SettingsService) SetLayout(ctx context.Context, patch settings.LayoutPatch) (domain.Settings, error) {
	return s.settings.SetLayout(ctx, patch)
}

func (s *SettingsService) SetRetention(ctx context.Context, retention domain.Retention) (domain.Settings, error) {
	return s.settings.SetRetention(ctx, retention)
}
