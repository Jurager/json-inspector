package wails

import (
	"context"
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/fx"

	"json-inspector/internal/infra/httpx"
	"json-inspector/internal/infra/sqlite"
	"json-inspector/internal/platform"
	"json-inspector/internal/usecase/environment"
	"json-inspector/internal/usecase/record"
	"json-inspector/internal/usecase/settings"
)

// Assets is what only main can embed: the built frontend and the window icon. go:embed reaches
// nothing outside the directory tree of the package that writes it.
type Assets struct {
	FS   fs.FS
	Icon []byte
}

// ServicesIn is the bound service set, and its field order is the lifecycle order: services start
// in this order and stop in the reverse, which is why the extension socket is last (first to go)
// and nothing here closes the database (that is an fx hook, and it runs after Run returns).
type ServicesIn struct {
	fx.In
	System       *SystemService
	Settings     *SettingsService
	Records      *RecordsService
	Environments *EnvironmentsService
	Bridge       *BridgeService
}

var Module = fx.Module("wails",
	fx.Provide(
		// Ports are bound here, in the composition layer: it is the only place that knows both a use
		// case and the adapter that serves it. Everything above this line is a constructor.
		func(store *sqlite.Store) environment.Store { return store },
		func(store *sqlite.Store) settings.Store { return store },
		func() environment.SecretSource { return keychainSecrets{} },
		func(host *Host) settings.Notifier { return newBus(host) },
		func(store *sqlite.Store) record.Store { return store },
		func(engine *httpx.Engine) record.Executor { return engineExecutor{engine: engine} },
		func(host *Host) record.Notifier { return newBus(host) },
		func(uc *settings.UseCase) record.RetentionSource { return settingsRetention(uc) },
		NewHost,
		NewStatus,
		openStorage,
		NewSystemService,
		NewSettingsService,
		NewRecordsService,
		NewEnvironmentsService,
		NewBridgeService,
		newCaptureIngest,
		newApplication,
	),
	fx.Invoke(setup),
)

// newApplication builds the Wails app. It is the only place application.New is called, and it
// creates no window: those come after the services are registered, so a window's runtime-ready
// event cannot arrive before the services that answer it exist.
func newApplication(host *Host, info platform.BuildInfo, assets Assets) *application.App {
	app := application.New(application.Options{
		Name:        info.Name,
		Description: info.Description,
		Icon:        assets.Icon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets.FS),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "com.jurager.json-inspector",
			EncryptionKey:          singleInstanceKey,
			OnSecondInstanceLaunch: host.OnSecondInstance,
		},
	})
	host.Attach(app)
	return app
}

// setup is the whole desktop side of assembly: services first, then the window, then the menu.
func setup(
	app *application.App,
	host *Host,
	status *Status,
	in ServicesIn,
	info platform.BuildInfo,
	settingsUC *settings.UseCase,
) {
	// The window's first paint happens before any frontend code runs, so the theme travels on its
	// URL rather than over IPC. Reading it here is safe: the database opened before this ran.
	if theme, err := settingsUC.Theme(context.Background()); err == nil {
		host.SetTheme(string(theme))
	}
	system := application.NewService(in.System)
	settingsService := application.NewService(in.Settings)
	recordsService := application.NewService(in.Records)
	environments := application.NewService(in.Environments)
	bridgeService := application.NewService(in.Bridge)

	app.RegisterService(system)
	app.RegisterService(settingsService)
	app.RegisterService(recordsService)
	// Without a database there is nothing else to offer, and a reduced surface is the difference
	// between a window that explains itself and one where half the controls fail. The frontend
	// reads StartupStatus and renders the failure instead of reaching for these.
	if status.Ready() {
		app.RegisterService(environments)
		app.RegisterService(bridgeService)
	}

	mainWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             windowMain,
		Title:            info.Name,
		Width:            1280,
		Height:           820,
		MinWidth:         900,
		MinHeight:        600,
		Frameless:        UseCustomTitlebar(),
		BackgroundColour: application.NewRGB(30, 30, 30),
		URL:              "/" + host.themeQuery(),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 50,
		},
	})
	host.SetMainWindow(mainWin)

	mainWin.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		host.MarkReady()
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
		host.HandleURLOpen(e.Context().URL())
	})

	if UseCustomTitlebar() {
		app.Menu.Set(app.NewMenu())
	} else {
		app.Menu.Set(BuildMenu(host, info.Name))
	}
}
