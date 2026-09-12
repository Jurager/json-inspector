package wails

import (
	"context"
	"fmt"
	"log"

	"json-inspector/internal/infra/sqlite"
	"json-inspector/migrations"
)

// InitFunc is the part of startup the frontend can retry: open the database and bring the schema
// up to date.
type InitFunc func(ctx context.Context) error

// openStorage builds that step and runs it once, here in the graph rather than in a lifecycle
// hook, because the outcome decides which services may be registered — and registration happens
// before the app runs. A failure is recorded rather than returned: returning it from a service's
// start-up aborts the run before any window exists, which is the blank window this avoids.
//
// The returned function is what the failure screen's "Повторить" calls; opening and migrating are
// both repeatable, so a retry after a locked file or a full disk is safe.
func openStorage(store *sqlite.Store, status *Status) InitFunc {
	init := func(ctx context.Context) error {
		if err := store.Open(ctx); err != nil {
			status.Fail(FailureDatabase, "Не удалось открыть базу данных", err.Error())
			return err
		}
		res, err := store.Migrate(ctx, migrations.FS)
		if err != nil {
			status.Fail(FailureMigration, "Не удалось обновить схему базы данных", err.Error())
			return err
		}
		if len(res.Applied) > 0 {
			applied := make([]string, 0, len(res.Applied))
			for _, m := range res.Applied {
				applied = append(applied, fmt.Sprintf("%04d_%s", m.Version, m.Name))
			}
			log.Printf("[storage] applied %d migration(s): %v", len(applied), applied)
		}
		status.Succeed()
		return nil
	}

	_ = init(context.Background())
	return init
}
