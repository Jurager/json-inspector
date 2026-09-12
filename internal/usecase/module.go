// Package usecase gathers the application's features. Each one owns a folder of its own; this file
// is the only place that lists them, so adding a feature is a folder and a line here.
package usecase

import (
	"go.uber.org/fx"

	"json-inspector/internal/usecase/environment"
)

var Module = fx.Module("usecase",
	environment.Module,
)
