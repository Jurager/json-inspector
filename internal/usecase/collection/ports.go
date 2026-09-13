package collection

import (
	"context"

	"json-inspector/internal/domain"
)

// Store is the tree as this feature keeps it. The whole thing is read at once — a collection is a
// document, not a list to page through — and written a node at a time. Runs are written beside it:
// a run row when it starts and again when it ends, and one row per request that was reached.
type Store interface {
	Collections(ctx context.Context) ([]domain.Collection, error)
	Node(ctx context.Context, id string) (domain.CollectionNode, error)

	SaveCollection(ctx context.Context, collection domain.Collection) error
	SaveNode(ctx context.Context, node domain.CollectionNode) error
	DeleteCollection(ctx context.Context, id string) error
	DeleteNode(ctx context.Context, id string) error

	// Scripts are read and written by the level they belong to, which is a collection or a node, and
	// the two are addressed in the same id space. Nil is "not set here" — the level inherits.
	Scripts(ctx context.Context, id string) (*domain.Scripts, error)
	SaveScripts(ctx context.Context, id string, scripts *domain.Scripts) error

	NextPosition(ctx context.Context, collectionID string, parentID string) (int64, error)

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
type Sender interface {
	Send(ctx context.Context, req RunRequest) (domain.Record, error)
}

// Notifier publishes what happened to whoever is listening.
type Notifier interface {
	Publish(topic string, payload any)
}
