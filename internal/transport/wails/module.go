package wails

import (
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/fx"

	"json-inspector/internal/platform"
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
	Requests     *RequestsService
	Environments *EnvironmentsService
	Bridge       *BridgeService
}

var Module = fx.Module("wails",
	fx.Provide(
		NewHost,
		NewStatus,
		openStorage,
		NewSystemService,
		NewRequestsService,
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
) {
	system := application.NewService(in.System)
	requests := application.NewService(in.Requests)
	environments := application.NewService(in.Environments)
	bridgeService := application.NewService(in.Bridge)

	app.RegisterService(system)
	// Without a database there is nothing else to offer, and a reduced surface is the difference
	// between a window that explains itself and one where half the controls fail. The frontend
	// reads StartupStatus and renders the failure instead of reaching for these.
	if status.Ready() {
		app.RegisterService(requests)
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
		URL:              "/",
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
