package environment

import "go.uber.org/fx"

var Module = fx.Module("usecase/environment",
	fx.Provide(NewUseCase),
)
