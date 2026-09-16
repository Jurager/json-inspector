package update

import "go.uber.org/fx"

var Module = fx.Module("usecase/update",
	fx.Provide(NewUseCase),
)
