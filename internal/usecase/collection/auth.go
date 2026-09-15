package collection

// What each level authorizes its requests with, and where a request finds the nearest level
// above it that answered: the walk the card and the run both ask.

import (
	"context"

	"json-inspector/internal/domain"
)

// SaveAuth writes what a level authorizes its requests with — the collection's own tab, or a
// folder's, which everything inside it inherits.
//
// What arrives is the tab's whole state, and the tab holds on to the answers of the schemes it is
// not currently on: «нет» chosen over a filled-in Bearer is an answer about who authorizes the
// request rather than an erasure, and coming back to Bearer has to find the token. It is
// domain.Auth.Stored that draws that distinction, which is why it lives there and not here.
func (u *UseCase) SaveAuth(
	ctx context.Context,
	id string,
	auth domain.Auth,
) ([]domain.Collection, error) {
	// Normalized for the same reason a draft's is: the answers travel whole, and the scheme they are
	// about may not be the one the previous answer was about.
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

// inheritUnder walks the tree looking for a request, carrying the answer of the levels above it:
// the nearest level that set one wins, and a level that set none passes down what it was given.
//
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

// answerOf is where a walk stands after looking at a level: the level's own answer when it gave
// one, and what it inherits when it did not. Both walks over the tree go through here, so «нет»
// means the same thing in a run as it does in a card — and a level that said it is a level passed
// by.
func answerOf(level *domain.Auth, inherited *domain.Auth) *domain.Auth {
	if level.Answers() {
		return level
	}
	return inherited
}

// actionable is a stored auth as something to apply: nothing to inherit is «нет», which is what a
// request with nothing above it authorizes itself with.
func actionable(auth *domain.Auth) domain.Auth {
	if auth == nil {
		return domain.NewAuth(domain.AuthNone)
	}
	return *auth
}

// storedAuth is an auth as the tree keeps one, for a caller that may not have given any at all. A
// pointer that is not there stays not there; one that says «нет» without an answer behind it is
// stored as nothing, which is what domain.Auth.Stored is for.
func storedAuth(auth *domain.Auth) *domain.Auth {
	if auth == nil {
		return nil
	}
	return auth.Stored()
}
