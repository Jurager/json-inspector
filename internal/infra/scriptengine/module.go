package scriptengine

import "go.uber.org/fx"

// Module provides the sandbox. The engine holds nothing between runs, so there is nothing to
// configure and nothing to shut down.
var Module = fx.Module("scriptengine",
	fx.Provide(NewEngine),
)
