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
	"json-inspector/internal/infra/updater"
	"json-inspector/internal/platform"
	"json-inspector/internal/transport/bridge"
	"json-inspector/internal/transport/wails"
	"json-inspector/internal/usecase"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

var version = "dev"

// build is the CI run number, empty for local builds; shown in brackets in About.
var build = ""

const (
	appName = "JSON Inspector"
	// The bundle's own description. It is a single field of the platform's manifest, set once at build
	// time, so it cannot follow the language the window is in — the app's name cannot either. Of the
	// two languages it could be written in, English is the one the manifest is read in.
	appDescription = "A JSON:API viewer: environment variables, a schema map, and captured " +
		"browser requests."

	// How long stopping the graph gets before the process leaves anyway: a hook that hangs — the
	// bridge socket waiting on a client — must not hold the window open after the user closed it.
	shutdownTimeout = 5 * time.Second
)

func main() {
	os.Exit(run())
}

// appOptions is the whole app as a dependency graph. It is a function of its own so the wiring can
// be checked without a window: a missing provider is otherwise a blank screen at startup.
func appOptions() []fx.Option {
	return []fx.Option{
		// Quiet, because a GUI process has no console on most platforms: the failures that matter
		// are reported through the window, and the ones that are not are logged by run.
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
			platform.BuildInfo{
				Version:     version,
				Build:       build,
				Name:        appName,
				Description: appDescription,
			},
			wails.Assets{FS: assets, Icon: appIcon},
			bridge.Port(bridge.DefaultPort),
			// The engine's defaults: env proxy, verified certificates, redirects followed, no jar, a
			// fresh connection per request. The settings screen is where these end up; the zero value
			// is what the app answers with until then.
			httpx.Config{},
		),
	}
}

// run is the composition root and nothing else: every decision about what the app is made of lives
// in the modules above, and every decision about their order lives in transport/wails.
func run() int {
	// The update checker reads these globals itself, so they are set before the graph exists; the
	// same values travel into the graph as BuildInfo for everything that only displays them.
	updater.CurrentVersion = version
	updater.CurrentBuild = build

	// SIGTERM is how a session manager stops the app — systemd, a container, a logout — and it is
	// not os.Interrupt, which is only what Ctrl-C sends. On Windows the signal is never delivered,
	// and asking for it costs nothing.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var app *application.App

	graph := fx.New(append(appOptions(), fx.Populate(&app))...)
	// Stopping after Run returns is what puts the database close last: fx hooks run on the way
	// out, and by then every service has already been shut down.
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
