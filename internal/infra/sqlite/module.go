package sqlite

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module("sqlite",
	fx.Provide(NewStore),
	fx.Invoke(closeOnStop),
)

// closeOnStop closes the file after the app has stopped its services: fx stops after Run returns,
// so the window and the extension socket are already gone and nothing can write into a closed
// database.
func closeOnStop(lc fx.Lifecycle, store *Store) {
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return store.Close()
		},
	})
}
