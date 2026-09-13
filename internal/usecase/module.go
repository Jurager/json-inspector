// Package usecase gathers the application's features. Each one owns a folder of its own; this file
// is the only place that lists them, so adding a feature is a folder and a line here.
package usecase

import (
	"go.uber.org/fx"

	"json-inspector/internal/usecase/environment"
	"json-inspector/internal/usecase/record"
	"json-inspector/internal/usecase/settings"
)

var Module = fx.Module("usecase",
	environment.Module,
	record.Module,
	settings.Module,
)
