package main

import (
	"context"
	"embed"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/fx"

	"json-inspector/internal/infra/files"
	"json-inspector/internal/infra/httpx"
	"json-inspector/internal/infra/scriptengine"
	"json-inspector/internal/infra/sqlite"
	"json-inspector/internal/platform"
	"json-inspector/internal/transport/bridge"
	"json-inspector/internal/transport/wails"
	"json-inspector/internal/usecase"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

const shutdownTimeout = 5 * time.Second

func main() {
	os.Exit(run())
}

func appOptions() []fx.Option {
	return []fx.Option{
		fx.NopLogger,
		platform.Module,
		sqlite.Module,
		httpx.Module,
		files.Module,
		scriptengine.Module,
		bridge.Module,
		usecase.Module,
		wails.Module,
		fx.Supply(
			wails.Assets{FS: assets, Icon: appIcon},
			bridge.Port(bridge.DefaultPort),
			httpx.Config{},
		),
	}
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var app *application.App

	graph := fx.New(append(appOptions(), fx.Populate(&app))...)

	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := graph.Stop(shutdown); err != nil {
			log.Printf("[app] shutdown: %v", err)
		}
	}()

	if err := graph.Err(); err != nil {
		log.Printf("[app] build failed: %v", err)
		return 1
	}
	if err := graph.Start(ctx); err != nil {
		log.Printf("[app] startup failed: %v", err)
		return 1
	}
	if err := app.Run(); err != nil {
		log.Printf("[app] %v", err)
		return 1
	}
	return 0
}
