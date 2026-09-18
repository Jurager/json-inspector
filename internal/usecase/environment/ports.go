package environment

import (
	"context"

	"json-inspector/internal/domain"
)

// Store is the persistence this feature needs, declared next to its user so the feature does not
// have to know whether the rows come from SQLite or a fake in a test. Every call names the
// workspace it works in: a name may be taken in each of them.
type Store interface {
	EnvState(ctx context.Context, workspaceID string) (domain.EnvState, error)
	SaveEnvironment(ctx context.Context, workspaceID string, env domain.Environment) error
	DeleteEnvironment(ctx context.Context, workspaceID, id string) error
	SaveVariable(ctx context.Context, workspaceID string, scope domain.EnvScope,
		v domain.Variable) error
	DeleteVariable(ctx context.Context, workspaceID, id string) error
	VariableValue(ctx context.Context, workspaceID, id string) (string, error)

	// Which environment the workspace is working in is kept on the workspace itself: two spaces are
	// two answers, and a switch that kept one would resolve against the other space's variables.
	SetActiveEnvironment(ctx context.Context, workspaceID, id string) error

	// The one-time import of what the old frontend kept in localStorage.
	ClaimImport(ctx context.Context, source string) (bool, error)
	FinishImport(ctx context.Context, source string, status domain.ImportStatus, detail string) error
}

// Scope answers which workspace the window is showing. It is a port of this feature's own rather
// than a call into the workspace use case: features never import each other, and this one only
// needs to know the name of the space it is working in.
type Scope interface {
	ActiveWorkspace(ctx context.Context) (string, error)
}

// Home names the workspace the installation started with. The one-time import of what the old
// frontend kept in localStorage repairs this installation's own data, so it lands there and not in
// whichever space happens to be on screen — and not on a fixed id either, now that the row the app
// is born with can be deleted like any other.
type Home interface {
	FirstWorkspace(ctx context.Context) (string, error)
}
