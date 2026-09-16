package record

import (
	"context"
	"log"

	"go.uber.org/fx"
)

var Module = fx.Module("usecase/record",
	fx.Provide(NewUseCase),
	fx.Invoke(pruneOnStart),
)

// pruneOnStart applies the retention rules once when the app comes up: a history that has not been
// touched for a while still ages, and the batch counter above only runs while records are being
// written.
func pruneOnStart(lc fx.Lifecycle, uc *UseCase) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if removed, err := uc.Prune(ctx); err != nil {
				log.Printf("[records] retention at startup: %v", err)
			} else if removed > 0 {
				log.Printf("[records] retention at startup dropped %d record(s)", removed)
			}
			return nil
		},
	})
}
