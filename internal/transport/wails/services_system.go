package wails

import (
	"context"
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/infra/updater"
	"json-inspector/internal/platform"
)

// SystemService is the app talking about itself: what it is, what its window does, when it last
// checked for a release, and whether it managed to open its own data.
type SystemService struct {
	host    *Host
	info    platform.BuildInfo
	status  *Status
	init    InitFunc
	dataDir platform.DataDir
}

func NewSystemService(
	host *Host,
	info platform.BuildInfo,
	status *Status,
	init InitFunc,
	dataDir platform.DataDir,
) *SystemService {
	return &SystemService{host: host, info: info, status: status, init: init, dataDir: dataDir}
}

func (s *SystemService) Version() string {
	return s.info.Version
}

// Build is the CI run number, empty for local builds.
func (s *SystemService) Build() string {
	return s.info.Build
}

func (s *SystemService) Name() string {
	return s.info.Name
}

func (s *SystemService) ToggleMaximize() {
	s.host.ToggleMaximize()
}

func (s *SystemService) ShowAbout() {
	s.host.ShowAbout()
}

// ApplyLanguage is the window handing over the words the native menu needs. The menu is drawn by the
// system rather than by the page, and Go cannot resolve "system" — that is a question only the webview
// can ask — so this is the one piece of the interface that is sent instead of read from the catalogue.
func (s *SystemService) ApplyLanguage(labels MenuLabels) {
	s.host.ApplyMenu(labels)
}

func (s *SystemService) ShowSettings() {
	s.host.ShowSettings()
}

// RequestUpdateCheck reuses or opens the About window and asks it to run a check.
func (s *SystemService) RequestUpdateCheck() {
	s.host.RequestUpdateCheck()
}

// TakeUpdateCheckRequest reports and clears a request parked by RequestUpdateCheck.
func (s *SystemService) TakeUpdateCheckRequest() bool {
	return s.host.TakeUpdateCheckRequest()
}

// CheckForUpdates answers the About window's own check. A release it finds is also announced to
// the main window, so the status bar's link appears without waiting for the next launch — the
// one place a manual check and the startup check have to agree.
func (s *SystemService) CheckForUpdates() (*updater.Info, error) {
	u, err := updater.Check()
	if err != nil {
		return nil, err
	}
	if u.Available {
		s.host.AnnounceUpdate(&u)
	}
	return &u, nil
}

// UpdateStatus is what the last check saw, without touching the network.
func (s *SystemService) UpdateStatus() (*updater.Info, error) {
	info, err := updater.Status()
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *SystemService) UpdateNow(version string) error {
	return updater.Install(version)
}

// StartupStatus is what the frontend reads first: without a database there is nothing else to
// show, and this is where it learns why.
func (s *SystemService) StartupStatus() StartupStatus {
	return StartupStatus{
		Ready:   s.status.Ready(),
		Failure: s.status.Failure(),
		DataDir: string(s.dataDir),
		DBPath:  s.dataDir.DatabasePath(),
	}
}

// RetryInit re-runs the startup step behind the failure screen's "Повторить" — the database may
// have been locked, the disk full, the directory permissions wrong.
func (s *SystemService) RetryInit() (StartupStatus, error) {
	err := s.init(context.Background())
	return s.StartupStatus(), err
}

// OpenDataFolder shows the user where the database lives, which is the one useful action left
// when it cannot be opened.
func (s *SystemService) OpenDataFolder() error {
	app := s.host.App()
	if app == nil {
		return errors.New("the application is not ready yet")
	}
	return app.Env.OpenFileManager(string(s.dataDir), false)
}

// StartupCheck is the launch-time update check, run once the window can take the answer.
func (s *SystemService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	go func() {
		if u := updater.StartupCheck(); u != nil {
			s.host.AnnounceUpdate(u)
		}
	}()
	return nil
}
