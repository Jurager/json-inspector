// Package collection owns the saved requests: the tree they live in, what inherits from what, and
// the runs that walk it.
package collection

import (
	"context"
	"sync/atomic"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

type UseCase struct {
	store       Store
	scope       Scope
	sender      Sender
	assertions  Assertions
	environment Environment
	notifier    Notifier
	ids         platform.IDGen

	// Only one run at a time: two of them would write their rows into the same overview.
	running atomic.Bool
	stopped atomic.Bool
}

func NewUseCase(
	store Store,
	scope Scope,
	sender Sender,
	assertions Assertions,
	environment Environment,
	notifier Notifier,
	ids platform.IDGen,
) *UseCase {
	return &UseCase{
		store:       store,
		scope:       scope,
		sender:      sender,
		assertions:  assertions,
		environment: environment,
		notifier:    notifier,
		ids:         ids,
	}
}

// Tree is every collection with its nodes, which is what the list draws and what a run walks.
func (u *UseCase) Tree(ctx context.Context) ([]domain.Collection, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// Node reads one node whole: everything opening a request needs, and the tree deliberately left
// out.
func (u *UseCase) Node(ctx context.Context, id string) (domain.CollectionNode, error) {
	return u.store.Node(ctx, id)
}

// LevelRows reads the requests a collection holds, as its page draws them: a name, a method and an
// address each. Only the level itself — the collections inside it are levels of their own, and
// their rows are read when one of them is opened.
func (u *UseCase) LevelRows(ctx context.Context, collectionID string) ([]domain.LevelRow, error) {
	return u.store.LevelRows(ctx, collectionID)
}

// findNode looks a request up in the tree, collections inside collections included: the tree is
// small enough to walk, and walking it is what says whether the id names a request at all.
func findNode(tree []domain.Collection, id string) (domain.CollectionNode, bool) {
	for _, collection := range tree {
		for _, node := range collection.Items {
			if node.ID == id {
				return node, true
			}
		}
		if node, ok := findNode(collection.Children, id); ok {
			return node, true
		}
	}
	return domain.CollectionNode{}, false
}

// collection finds a collection by id and says whether it found one rather than failing: nodes are
// addressed in the same id space, and a caller naming an id does not say which of the two it named.
func (u *UseCase) collection(
	ctx context.Context,
	workspace, id string,
) (domain.Collection, bool, error) {
	tree, err := u.store.Collections(ctx, workspace)
	if err != nil {
		return domain.Collection{}, false, err
	}
	collection, ok := findCollection(tree, id)
	return collection, ok, nil
}
