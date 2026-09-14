package wails

import (
	"context"
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/fx"

	"json-inspector/internal/domain"
	"json-inspector/internal/infra/files"
	"json-inspector/internal/infra/httpx"
	"json-inspector/internal/infra/scriptengine"
	"json-inspector/internal/infra/sqlite"
	"json-inspector/internal/platform"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/draft"
	"json-inspector/internal/usecase/environment"
	"json-inspector/internal/usecase/record"
	"json-inspector/internal/usecase/scripting"
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
	Drafts       *DraftService
	Environments *EnvironmentsService
	Collections  *CollectionsService
	Scripting    *ScriptingService
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
		func(store *sqlite.Store) draft.Store { return store },
		func(store *sqlite.Store) collection.Store { return store },
		func(host *Host) collection.Notifier { return newBus(host) },
		func(drafts *draft.UseCase, records *record.UseCase) collection.Sender {
			return collectionSender{drafts: drafts, records: records}
		},
		func(engine *httpx.Engine) record.Executor { return engineExecutor{engine: engine} },
		func(host *Host) record.Notifier { return newBus(host) },
		func(uc *settings.UseCase) record.RetentionSource { return settingsRetention(uc) },
		func(uc *environment.UseCase) draft.VariableSource { return environmentVariables{uc} },
		func(r *files.Reader) draft.FileSource { return r },
		func(engine *scriptengine.Engine) scripting.Engine { return engine },
		func(store *sqlite.Store) scripting.Tree { return store },
		func(store *sqlite.Store) scripting.Store { return store },
		func(uc *environment.UseCase) scripting.Variables { return uc },
		// The scripts around a request are asked by the feature that sends it, and they are told what
		// to run by the feature that keeps them. Neither knows the other; this is the pair.
		func(scripts *scripting.UseCase) record.Screener { return scripts },
		func(drafts *draft.UseCase) record.Masker { return requestMask{drafts} },
		NewHost,
		NewStatus,
		openStorage,
		NewSystemService,
		NewSettingsService,
		NewRecordsService,
		NewDraftService,
		NewEnvironmentsService,
		NewCollectionsService,
		NewScriptingService,
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
		// A refused call carries its code beside its message, so the window can word the refusal in the
		// language it is in. This is the only hook that reaches every bound method at once.
		MarshalError: marshalFailure,
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "com.jurager.json-inspector",
			EncryptionKey:          singleInstanceKey,
			OnSecondInstanceLaunch: host.OnSecondInstance,
		},
	})
	// The name a window is titled with until its page draws.
	host.SetAppName(info.Name)
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
	// URL rather than over IPC. Reading it here is safe: the database opened before this ran. The
	// same value is what the native glass material is tinted by, and that one can only be told to
	// the window as it is created.
	theme, err := settingsUC.Theme(context.Background())
	if err != nil {
		theme = domain.ThemeSystem
	}
	host.SetTheme(theme)
	// The language travels the same road and for the same reason: the first frame is already written.
	language, err := settingsUC.Language(context.Background())
	if err != nil {
		language = domain.LanguageSystem
	}
	host.SetLanguage(language)
	system := application.NewService(in.System)
	settingsService := application.NewService(in.Settings)
	recordsService := application.NewService(in.Records)
	draftService := application.NewService(in.Drafts)
	environments := application.NewService(in.Environments)
	collections := application.NewService(in.Collections)
	scripting := application.NewService(in.Scripting)
	bridgeService := application.NewService(in.Bridge)

	app.RegisterService(system)
	app.RegisterService(settingsService)
	app.RegisterService(recordsService)
	app.RegisterService(draftService)
	// Without a database there is nothing else to offer, and a reduced surface is the difference
	// between a window that explains itself and one where half the controls fail. The frontend
	// reads StartupStatus and renders the failure instead of reaching for these.
	if status.Ready() {
		app.RegisterService(environments)
		app.RegisterService(collections)
		app.RegisterService(scripting)
		app.RegisterService(bridgeService)
	}

	// The window is translucent: what the chrome and the overlays paint is a glass material, and the
	// material has nothing to show unless the window itself lets the platform draw behind it.
	mainWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      windowMain,
		Title:     info.Name,
		Width:     1280,
		Height:    820,
		MinWidth:  900,
		MinHeight: 600,
		Frameless: UseCustomTitlebar(),
		// A fully transparent background: the page paints the ground of its content itself, and every
		// other pixel is the material behind the window.
		BackgroundType:   glassBackgroundType(),
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		URL:              "/" + host.windowQuery(glassShows()),
		Mac: application.MacWindow{
			Backdrop:                glassMacBackdrop(),
			Appearance:              macWindowAppearance(theme),
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 50,
		},
		Windows: application.WindowsWindow{
			BackdropType: glassWindowsBackdrop(),
			Theme:        windowTheme(theme),
		},
	})
	host.SetMainWindow(mainWin)

	mainWin.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		// The window that shows the material behind it has to let it through — on a platform with no
		// material this is nothing. It waits for this event because the call reaches the window through
		// the main thread, and before the app runs there is no main thread to reach it on.
		prepareGlassWindow(mainWin, theme)
		host.MarkReady()
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
		host.HandleURLOpen(e.Context().URL())
	})
	// While the app follows the system, the OS switching its own theme is a theme change like any
	// other — and one the frontend cannot pass on, since the material is not the page's to move.
	app.Event.OnApplicationEvent(events.Common.ThemeChanged, func(*application.ApplicationEvent) {
		host.SystemThemeChanged()
	})

	// The menu's own words are not known yet — "system" is a question only the webview can answer, and
	// it answers once the page is up. So the menu is built here with the fallback language and built
	// again by ApplyLanguage, which the window calls as it mounts.
	if UseCustomTitlebar() {
		app.Menu.Set(app.NewMenu())
	} else {
		app.Menu.Set(BuildMenu(host, info.Name, MenuLabels{
			About:        "About",
			Help:         "Help",
			CheckUpdates: "Check for updates…",
		}))
	}
}
