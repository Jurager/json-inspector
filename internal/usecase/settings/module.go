package settings

import "go.uber.org/fx"

var Module = fx.Module("usecase/settings",
	fx.Provide(NewUseCase),
)
