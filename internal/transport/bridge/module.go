package bridge

import "go.uber.org/fx"

// Module provides the extension endpoint. The port and the ingest both come from outside: the
// port is the composition root's decision, and the ingest is implemented a layer up.
var Module = fx.Module("bridge",
	fx.Provide(NewServer),
)
