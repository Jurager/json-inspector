// Package usecase gathers the application's features. Each one owns a folder of its own; this file
// is the only place that lists them, so adding a feature is a folder and a line here.
package usecase

import (
	"go.uber.org/fx"

	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/draft"
	"json-inspector/internal/usecase/environment"
	"json-inspector/internal/usecase/record"
	"json-inspector/internal/usecase/scripting"
	"json-inspector/internal/usecase/settings"
)

var Module = fx.Module("usecase",
	collection.Module,
	draft.Module,
	environment.Module,
	record.Module,
	scripting.Module,
	settings.Module,
)
