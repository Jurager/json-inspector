package scripting

import "go.uber.org/fx"

// Module provides the scripts of a collection. What it runs them around is not its business: the
// feature that sends a request asks it before and after, and the composition root is where the two
// meet.
var Module = fx.Module("scripting",
	fx.Provide(NewUseCase),
)
