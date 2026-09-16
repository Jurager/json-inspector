package wails

import (
	"context"
	"errors"

	"json-inspector/internal/domain"
	"json-inspector/internal/infra/authflow"
	"json-inspector/internal/platform"
)

// SystemService is the app talking about itself: what it is, what its windows do, and whether it
// managed to open its own data. Updating has a service of its own; everything that belongs to no
// feature in particular stays here.
type SystemService struct {
	host    *Host
	info    platform.BuildInfo
	status  *Status
	init    InitFunc
	dataDir platform.DataDir
	// auth is here for the words it cannot look up itself: see ApplyLanguage.
	auth *authflow.Materializer
}

func NewSystemService(
	host *Host,
	info platform.BuildInfo,
	status *Status,
	init InitFunc,
	dataDir platform.DataDir,
	auth *authflow.Materializer,
) *SystemService {
	return &SystemService{host: host, info: info, status: status, init: init, dataDir: dataDir,
		auth: auth}
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

// signInPagesFallback is what a sign-in puts in the browser before the window has said anything.
// English, because that is the language the build's own words are in; the window's choice replaces
// it as it mounts, and the person reading a sign-in page is almost always looking at one that
// arrived after that.
var signInPagesFallback = domain.SignInPages{
	Waiting: domain.SignInPage{
		Title: "Done",
		Text:  "Handing the token to the application…",
	},
	Done: domain.SignInPage{
		Title: "Done",
		Text:  "You can close this tab and go back to the application.",
	},
	Refused: domain.SignInPage{
		Title: "Refused",
		Text:  "The provider did not grant access. Go back to the application.",
	},
	Failed: "Could not hand the token to the application.",
}

// ApplyLanguage is the window handing over the words of the two surfaces Go draws but does not own:
// the native menu, which the system draws, and the pages a sign-in puts in the browser. Neither can
// be worded here — the catalogue is the window's, and Go cannot resolve "system", which is a
// question only the webview can ask — so this is the one piece of the interface that is sent
// instead of read. The two travel together because they are needed at the same two moments: as the
// window mounts, and again whenever the language moves.
func (s *SystemService) ApplyLanguage(labels MenuLabels, pages domain.SignInPages) {
	s.host.ApplyMenu(labels)
	s.auth.SetPages(pages)
}

func (s *SystemService) ShowSettings() {
	s.host.ShowSettings()
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

// RetryInit re-runs the startup step behind the failure screen's "Retry" — the database may
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
