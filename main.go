package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"json-inspector/internal/bridge"
	"json-inspector/internal/update"
)

//go:embed all:frontend/dist
var assets embed.FS

// The app icon, handed to the OS (macOS dock/About, Windows taskbar).
//
//go:embed build/appicon.png
var appIcon []byte

// version is overridden at build time via -ldflags "-X main.version=…", which
// the per-OS BUILD_FLAGS in build/*/Taskfile.yml inject.
var version = "dev"

func main() {
	update.Current = version

	appService := NewApp()

	app := application.New(application.Options{
		Name:        product.Name,
		Description: product.Description,
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// Without this a second launch (or a json-inspector:// link while the app
		// is already running) starts a whole new process, which then fails to
		// bind the bridge port and sits there with a dead extension socket.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "com.jurager.json-inspector",
			EncryptionKey:          singleInstanceKey,
			OnSecondInstanceLaunch: appService.onSecondInstance,
		},
	})
	appService.setApp(app)

	mainWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             windowMain,
		Title:            product.Name,
		Width:            1280,
		Height:           820,
		MinWidth:         900,
		MinHeight:        600,
		Frameless:        useCustomTitlebar(),
		BackgroundColour: application.NewRGB(30, 30, 30),
		URL:              "/",
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 50,
		},
	})
	appService.setMainWindow(mainWin)

	// The frontend subscribes to events while mounting; anything emitted before
	// that is dropped, so this is the moment the buffered payloads can go out.
	mainWin.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		appService.markReady()
	})

	// First launch via json-inspector://. Later launches arrive through
	// SingleInstance.OnSecondInstanceLaunch instead — see handleUrlOpen.
	app.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
		appService.handleUrlOpen(e.Context().URL())
	})

	bridgeServer := bridge.NewServer(
		bridge.DefaultPort,
		appService.onCapturedRequest,
		appService.onCaptureState,
		appService.onCaptureDisconnected,
		appService.onFocusRequest,
	)
	appService.setBridge(bridgeServer)
	go func() {
		if err := bridgeServer.Start(); err != nil {
			log.Printf("[bridge] error: %v", err)
		}
	}()

	// The native app menu is only used on macOS, where it renders in the
	// system-wide menu bar and looks native. On Windows/Linux the app is
	// frameless and draws its own title bar; an empty menu keeps Wails from
	// installing its default one there.
	if useCustomTitlebar() {
		app.Menu.Set(app.NewMenu())
	} else {
		app.Menu.Set(buildMenu(appService))
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
