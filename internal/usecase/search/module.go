package search

import "go.uber.org/fx"

var Module = fx.Module("usecase/search",
	fx.Provide(NewUseCase),
)
