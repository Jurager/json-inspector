package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"json-inspector/internal/bridge"
	"json-inspector/internal/update"
)

var assets embed.FS
var appIcon []byte
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

	mainWin.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		appService.markReady()
	})

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

	// The native app menu is only used on macOS.
	// Where it renders in the system-wide menu bar and looks native.
	// On Windows/Linux the app is frameless and draws its own title bar.
	if useCustomTitlebar() {
		app.Menu.Set(app.NewMenu())
	} else {
		app.Menu.Set(buildMenu(appService))
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
