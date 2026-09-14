package collection

import (
	"context"

	"json-inspector/internal/domain"
)

// Store is the tree as this feature keeps it. The whole thing is read at once — a collection is a
// document, not a list to page through — and written a node at a time. Runs are written beside it:
// a run row when it starts and again when it ends, and one row per request that was reached.
//
// The tree, and everything that changes its shape, is told which workspace it is working in: a
// collection belongs to one, and the level a row is dropped into is a level of that workspace. A
// node is reached by its own id, which is unique across the database.
type Store interface {
	Collections(ctx context.Context, workspaceID string) ([]domain.Collection, error)
	Node(ctx context.Context, id string) (domain.CollectionNode, error)

	SaveCollection(ctx context.Context, workspaceID string, collection domain.Collection) error
	SaveNode(ctx context.Context, node domain.CollectionNode) error
	DeleteCollection(ctx context.Context, workspaceID, id string) error
	DeleteNode(ctx context.Context, id string) error

	// Scripts are read and written by the level they belong to, which is a collection or a request,
	// and the two are addressed in the same id space — as is the command line's own draft, which is
	// why the workspace has to be named here: the draft's id is the same word in every workspace.
	// Nil is "not set here" — the level inherits.
	Scripts(ctx context.Context, workspaceID, id string) (*domain.Scripts, error)
	SaveScripts(ctx context.Context, workspaceID, id string, scripts *domain.Scripts) error

	NextPosition(ctx context.Context, workspaceID, collectionID string) (int64, error)

	// Moving is a place in a level, counted the way the tree is drawn: the requests of a collection
	// and the collections inside it are one list, and the position is an index in it.
	MoveNode(ctx context.Context, workspaceID, id string, collectionID string, position int64) error
	MoveCollection(ctx context.Context, workspaceID, id string, parentID string, position int64) error

	SaveRun(ctx context.Context, run domain.CollectionRun) error
	AppendRunResult(ctx context.Context, runID string, result domain.CollectionRunResult) error
	LastRun(ctx context.Context, collectionID string, nodeID string) (domain.CollectionRun, bool, error)
}

// RunRequest is one saved request on its way out: what a node asks for, with its rows already
// narrowed to the ones that are switched on and its jar carried as rows.
//
// The run and the node travel with it although a run does not use them itself: a request that came
// from a collection has the scripts of everything above it around it, and whoever sends it has to be
// able to say which node it was.
type RunRequest struct {
	Run    string
	NodeID string

	Method  string
	URL     string
	Body    string
	Headers []domain.HeaderPair
	Cookies []domain.CookieRow

	// Auth is what the request inherits: the answer of the nearest level above it that gave one, or
	// nothing when none did. It travels resolved because the walk up the tree is the run's own —
	// by the time a request is on its way out, where it sits is no longer known.
	Auth *domain.Auth
}

// Sender sends one saved request and answers with what it produced. A run does not know how a
// request leaves the process — it goes out the way the command line's does, through the draft that
// fills in its `{{tokens}}` and keeps a secret's value out of what is written down.
// Scope answers which workspace the window is showing. It is a port of this feature's own rather
// than a call into the workspace use case: features never import each other, and this one only
// needs the name of the space it is working in.
type Scope interface {
	ActiveWorkspace(ctx context.Context) (string, error)
}

type Sender interface {
	Send(ctx context.Context, req RunRequest) (domain.Record, error)
}

// Notifier publishes what happened to whoever is listening.
type Notifier interface {
	Publish(topic string, payload any)
}
