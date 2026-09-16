package workspace

import "go.uber.org/fx"

var Module = fx.Module("usecase/workspace",
	fx.Provide(NewUseCase),
)
