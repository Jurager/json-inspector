package httpx

import "go.uber.org/fx"

// Module provides the engine. Its Config is supplied by the composition root — today the zero
// value, later whatever the settings screen says — so there is exactly one place to look for how
// the app talks to the network.
var Module = fx.Module("httpx",
	fx.Provide(NewEngine),
)
