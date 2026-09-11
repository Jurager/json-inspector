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

var version = "dev"

const (
	appName        = "JSON Inspector"
	appDescription = "Просмотр JSON:API: подстановка переменных окружения, карта схемы, перехват запросов из браузера."
)

func main() {
	update.Current = version

	appService := NewApp()

	app := application.New(application.Options{
		Name:        appName,
		Description: appDescription,
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
		Title:            appName,
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

	if useCustomTitlebar() {
		app.Menu.Set(app.NewMenu())
	} else {
		app.Menu.Set(buildMenu(appService))
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
