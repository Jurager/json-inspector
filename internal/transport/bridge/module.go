package bridge

import "go.uber.org/fx"

var Module = fx.Module("bridge",
	fx.Provide(NewServer),
)
