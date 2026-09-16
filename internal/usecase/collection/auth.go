package collection

// The auth walk the card and the run both ask: the nearest level above a request that answered.

import (
	"context"

	"json-inspector/internal/domain"
)

// SaveAuth writes what a level authorizes its requests with. The tab sends its whole state, so
// «None» over a filled-in Bearer is an answer about who authorizes, not an erasure — that
// distinction is domain.Auth.Stored's to draw, which is why it lives there.
func (u *UseCase) SaveAuth(
	ctx context.Context,
	id string,
	auth domain.Auth,
) ([]domain.Collection, error) {
	// Normalized like a draft's: the answers arrive whole, and the scheme may have changed.
	saved := auth.Normalized().Stored()

	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	if collection, ok, err := u.collection(ctx, workspace, id); err != nil {
		return nil, err
	} else if ok {
		collection.Auth = saved
		if err := u.store.SaveCollection(ctx, workspace, collection); err != nil {
			return nil, err
		}
		return u.store.Collections(ctx, workspace)
	}

	node, err := u.store.Node(ctx, id)
	if err != nil {
		return nil, err
	}
	node.Auth = saved
	if err := u.store.SaveNode(ctx, node); err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// AuthFor is what a request inherits: the answer of the nearest level above it that gave one, or
// nothing when none did. The command line's draft is in no tree, and a node that has been deleted
// is in none either — neither is a failure, because "there is nothing above this request" is what
// the answer is, not that the question was wrong.
func (u *UseCase) AuthFor(ctx context.Context, id domain.DraftID) (*domain.Auth, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	tree, err := u.store.Collections(ctx, workspace)
	if err != nil {
		return nil, err
	}
	for _, collection := range tree {
		if inherited, ok := inheritUnder([]domain.Collection{collection}, string(id), nil); ok {
			return actionable(inherited).Stored(), nil
		}
	}
	return nil, nil
}

// A collection is a level like any other, which is why the walk descends into the collections
// inside one and not only into its requests.
func inheritUnder(
	collections []domain.Collection,
	id string,
	inherited *domain.Auth,
) (*domain.Auth, bool) {
	for _, collection := range collections {
		at := answerOf(collection.Auth, inherited)
		for _, node := range collection.Items {
			nodeAt := answerOf(node.Auth, at)
			if node.ID == id {
				return nodeAt, true
			}
		}
		if found, ok := inheritUnder(collection.Children, id, at); ok {
			return found, true
		}
	}
	return nil, false
}

// Both walks over the tree go through here, so «None» means the same thing in a run as in a card.
func answerOf(level *domain.Auth, inherited *domain.Auth) *domain.Auth {
	if level.Answered() {
		return level
	}
	return inherited
}

// A request with nothing above it authorizes itself with «None».
func actionable(auth *domain.Auth) domain.Auth {
	if auth == nil {
		return domain.NewAuth(domain.AuthNone)
	}
	return *auth
}

// A nil auth stays nil — a level nobody touched; «None» with no answers behind it is stored as
// nothing, so the level still inherits.
func storedAuth(auth *domain.Auth) *domain.Auth {
	if auth == nil {
		return nil
	}
	return auth.Stored()
}
