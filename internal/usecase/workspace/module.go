package workspace

import (
	"context"
	"log"

	"go.uber.org/fx"
)

var Module = fx.Module("usecase/workspace",
	fx.Provide(NewUseCase),
	fx.Invoke(startOnLaunch),
)

// startOnLaunch settles which space the app opens in before any window asks. A failure is logged
// and not returned: the pointer falls back to the oldest space on its own, so a launch that could
// not read the preference behaves as if it had been left on — which is what it was.
func startOnLaunch(lc fx.Lifecycle, uc *UseCase) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := uc.Start(ctx); err != nil {
				log.Printf("[workspace] startup: %v", err)
			}
			return nil
		},
	})
}
