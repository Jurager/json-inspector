package collection

import "go.uber.org/fx"

var Module = fx.Module("usecase/collection",
	fx.Provide(NewUseCase),
)
