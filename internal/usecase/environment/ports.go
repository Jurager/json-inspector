package environment

import (
	"context"

	"json-inspector/internal/domain"
)

// Store is the persistence this feature needs. It is declared here, next to its user, so the
// feature does not have to know whether the rows come from SQLite or a fake in a test.
type Store interface {
	EnvState(ctx context.Context) (domain.EnvState, error)
	SaveEnvironment(ctx context.Context, env domain.Environment) error
	DeleteEnvironment(ctx context.Context, id string) error
	SaveVariable(ctx context.Context, scope domain.EnvScope, v domain.Variable) error
	DeleteVariable(ctx context.Context, id string) error
	VariableValue(ctx context.Context, id string) (string, error)
	SetActiveEnvironment(ctx context.Context, id string) error

	// The one-time import of what the old frontend kept in localStorage.
	ClaimImport(ctx context.Context, source string) (bool, error)
	FinishImport(ctx context.Context, source, status, detail string) error
}

// SecretSource is the OS keychain, which is where secrets lived before they moved into the
// database. Only the import reads from it, and only until that import has run everywhere.
type SecretSource interface {
	Get(ctx context.Context, scope, name string) (string, error)
}
