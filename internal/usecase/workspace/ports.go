package workspace

import (
	"context"

	"json-inspector/internal/domain"
)

// Store is the persistence this feature needs. It is declared here, next to its user, so the
// feature does not have to know whether the rows come from SQLite or a fake in a test.
type Store interface {
	Workspaces(ctx context.Context) ([]domain.Workspace, error)
	Workspace(ctx context.Context, id string) (domain.Workspace, error)
	SaveWorkspace(ctx context.Context, w domain.Workspace) error
	DeleteWorkspace(ctx context.Context, id string) error

	// The pointer to the workspace on screen, and the one setting that names it. It is a
	// preference of this installation rather than data of a workspace, which is why it lives in
	// the settings table and not on a row here.
	ActiveWorkspace(ctx context.Context) (string, error)
	SetActiveWorkspace(ctx context.Context, id string) error
}

// Notifier is how every window hears that the workspace on screen moved. The switcher is the only
// caller, but the event is a broadcast: the news is not for the window that asked.
type Notifier interface {
	Publish(topic string, payload any)
}
