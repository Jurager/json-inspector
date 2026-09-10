package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

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

	bridgeServer := bridge.NewServer(bridge.DefaultPort, app.onCapturedRequest)
	go func() {
		if err := bridgeServer.Start(); err != nil {
			log.Printf("[bridge] error: %v", err)
		}
	}()

	err := wails.Run(&options.App{
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
		Menu:             buildMenu(app),
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
