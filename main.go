package main

import (
	"embed"
	"log"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"json-inspector/internal/bridge"
	"json-inspector/internal/update"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is overridden at build time via -ldflags "-X main.version=…".
var version = "dev"

func main() {
	update.Current = version

	app := NewApp()

	bridgeServer := bridge.NewServer(bridge.DefaultPort, app.onCapturedRequest, app.onCaptureState, app.onCaptureDisconnected)
	app.setBridge(bridgeServer)
	go func() {
		if err := bridgeServer.Start(); err != nil {
			log.Printf("[bridge] error: %v", err)
		}
	}()

	// Neither Windows nor Linux have an equivalent of macOS's hidden-inset
	// title bar, and the native menu strip Wails would otherwise draw doesn't
	// follow the app's theme there (it renders as a plain, unstyled bar). So
	// on those platforms we go fully frameless and let the frontend draw its
	// own title bar, complete with its own caption buttons; the native app
	// menu is only used on macOS, where it renders in the system-wide menu
	// bar and looks native.
	useCustomTitlebar := goruntime.GOOS == "windows" || goruntime.GOOS == "linux"

	appOptions := &options.App{
		Title:     "JSON Inspector",
		Width:     1280,
		Height:    820,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 30, G: 30, B: 30, A: 1},
		OnStartup:        app.startup,
		Frameless:        useCustomTitlebar,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Linux: &linux.Options{
			WindowIsTranslucent: false,
		},
		Mac: &mac.Options{
			TitleBar:  mac.TitleBarHiddenInset(),
			OnUrlOpen: app.handleUrlOpen,
		},
	}

	if !useCustomTitlebar {
		appOptions.Menu = buildMenu(app)
	}

	err := wails.Run(appOptions)

	if err != nil {
		println("Error:", err.Error())
	}
}
