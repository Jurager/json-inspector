package wails

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/usecase/update"
)

// UpdateService is the update feature's whole surface towards a window: the About window and the
// settings window check, the main window's status bar reads what the last check found, and the
// update window installs it.
type UpdateService struct {
	update *update.UseCase
	host   *Host
}

func NewUpdateService(uc *update.UseCase, host *Host) *UpdateService {
	return &UpdateService{update: uc, host: host}
}

// Check runs a check the user asked for. Unlike the launch-time one it is never skipped and never
// gated on the preference, and what it found is published to every window: without that the status
// bar's link would not appear until the next launch.
func (s *UpdateService) Check(ctx context.Context) (update.Info, error) {
	info, err := s.update.Check(ctx)
	if err != nil {
		return update.Info{}, err
	}
	s.host.PublishUpdate(&info)
	return info, nil
}

// Status is what the last check saw, without touching the network: what a window opens on, and what
// the status bar is rebuilt from after a restart.
func (s *UpdateService) Status(ctx context.Context) (update.Info, error) {
	return s.update.Status(ctx)
}

// Skip records that the user does not want to hear about the version on screen again, answers with
// what the app has to say now, and publishes it: the window that asked is about to close, and the
// ones that stay open are showing the release it just dropped.
func (s *UpdateService) Skip(ctx context.Context) (update.Info, error) {
	info, err := s.update.Skip(ctx)
	if err != nil {
		return update.Info{}, err
	}
	s.host.PublishUpdate(&info)
	return info, nil
}

// Install downloads the release and replaces the running binary. On success it does not return.
func (s *UpdateService) Install(ctx context.Context, version string) error {
	return s.update.Install(ctx, version)
}

// ShowWindow opens the window that describes a release and offers to install it.
func (s *UpdateService) ShowWindow() {
	s.host.ShowUpdate()
}

// RequestCheck opens the About window and asks it to run a check. The menu and the rail both come
// here: the request is parked as well as sent, because a window that does not exist yet has nothing
// listening, and an open one never mounts again.
func (s *UpdateService) RequestCheck() {
	s.host.RequestUpdateCheck()
}

// TakeRequest reports and clears a check parked by RequestCheck. Destructive on purpose: both the
// mount and the window's own button call it, and only the first may act on it.
func (s *UpdateService) TakeRequest() bool {
	return s.host.TakeUpdateCheckRequest()
}

// ServiceStartup starts the launch-time check on a goroutine of its own: it reaches the network,
// and a window waiting for it would open seconds late. The answer is parked until the page can
// take an event.
func (s *UpdateService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	go func() {
		if info := s.update.StartupCheck(context.Background()); info != nil {
			s.host.PublishUpdate(info)
		}
	}()
	return nil
}
