package account

import "go.uber.org/fx"

var Module = fx.Module("usecase/account",
	fx.Provide(NewUseCase),
)
