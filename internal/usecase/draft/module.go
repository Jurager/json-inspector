package draft

import (
	"context"
	"log"

	"go.uber.org/fx"
)

var Module = fx.Module("usecase/draft",
	fx.Provide(NewUseCase),
	fx.Invoke(loadOnStart),
)

// loadOnStart reads the draft the last run left behind, before any window can ask for it: a request
// half composed is the one thing a user expects to still be there.
func loadOnStart(lc fx.Lifecycle, uc *UseCase) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := uc.Load(ctx); err != nil {
				// A draft that cannot be read is not a reason to hold the app back: the window
				// gets a fresh one, and the next change writes it.
				log.Printf("[draft] reading the stored draft: %v", err)
			}
			return nil
		},
	})
}
