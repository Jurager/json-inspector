package collection

import (
	"context"
	"testing"

	"json-inspector/internal/domain"
)

// authTree is a collection with another one inside it, a request inside that and one beside it: the
// shape that says whether a request finds the level that answers for it.
type authTree struct {
	uc           *UseCase
	collectionID string
	nestedID     string
	insideID     string
	besideID     string
}

func setupAuthTree(t *testing.T) authTree {
	t.Helper()
	uc, _ := newTestUseCase()
	ctx := context.Background()

	tree, err := uc.CreateCollection(ctx, "Коллекция", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	collectionID := only(t, tree).ID
	nestedID := nestCollection(t, uc, "Вложенная", collectionID)

	_, tree, err = uc.CreateNode(ctx, NewNode{CollectionID: nestedID, Name: "Внутри"})
	if err != nil {
		t.Fatalf("CreateNode inside: %v", err)
	}
	insideID := findInTree(t, tree, "Внутри").ID

	_, tree, err = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Name: "Рядом"})
	if err != nil {
		t.Fatalf("CreateNode beside: %v", err)
	}
	besideID := findInTree(t, tree, "Рядом").ID

	return authTree{uc: uc, collectionID: collectionID, nestedID: nestedID, insideID: insideID, besideID: besideID}
}

func bearer(token string) domain.Auth {
	return domain.NewAuth(domain.AuthBearer).With("token", token)
}

// bearerRef is the same answer where the tree holds one by pointer. Nil there means «nothing said
// here» — a level whose requests inherit past it — which is a different thing from «нет».
func bearerRef(token string) *domain.Auth {
	auth := bearer(token)
	return &auth
}

// What a level authorizes its requests with, and where a request finds it: the nearest level above
// it that answered, which is the collection inside one for what it holds and the outer collection
// for what is not in it.
func TestAuthIsInheritedDownTheTree(t *testing.T) {
	ctx := context.Background()
	a := setupAuthTree(t)

	if _, err := a.uc.SaveAuth(ctx, a.collectionID, bearer("коллекция")); err != nil {
		t.Fatalf("SaveAuth collection: %v", err)
	}

	got, err := a.uc.AuthFor(ctx, domain.DraftID(a.insideID))
	if err != nil {
		t.Fatalf("AuthFor: %v", err)
	}
	if got == nil || got.Get("token") != "коллекция" {
		t.Errorf("inherited = %+v, want the collection's", got)
	}

	// A collection that answers for itself stops the walk for everything inside it, and only for that.
	if _, err := a.uc.SaveAuth(ctx, a.nestedID, bearer("вложенная")); err != nil {
		t.Fatalf("SaveAuth nested: %v", err)
	}
	if got, _ := a.uc.AuthFor(ctx, domain.DraftID(a.insideID)); got == nil || got.Get("token") != "вложенная" {
		t.Errorf("inside inherited = %+v, want the nested collection's", got)
	}
	if got, _ := a.uc.AuthFor(ctx, domain.DraftID(a.besideID)); got == nil || got.Get("token") != "коллекция" {
		t.Errorf("beside inherited = %+v, want the collection's", got)
	}

	// A request that answers for itself is not the nested collection's any more.
	if _, err := a.uc.SaveAuth(ctx, a.insideID, bearer("сам")); err != nil {
		t.Fatalf("SaveAuth request: %v", err)
	}
	if got, _ := a.uc.AuthFor(ctx, domain.DraftID(a.insideID)); got == nil || got.Get("token") != "сам" {
		t.Errorf("own = %+v, want the request's own", got)
	}

	// «Нет» is not an answer that stops the walk: a level with nothing of its own lets the one below
	// it inherit from the one above.
	if _, err := a.uc.SaveAuth(ctx, a.nestedID, domain.Auth{Type: domain.AuthNone}); err != nil {
		t.Fatalf("SaveAuth nested none: %v", err)
	}
	if got, _ := a.uc.AuthFor(ctx, domain.DraftID(a.besideID)); got == nil || got.Get("token") != "коллекция" {
		t.Errorf("after clearing the nested one = %+v, want the collection's still", got)
	}
	tree, err := a.uc.Tree(ctx)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if nested := findIn(t, tree, a.nestedID); nested.Auth != nil {
		t.Errorf("nested auth = %+v, want it stored as nothing at all", nested.Auth)
	}
}

// The command line's draft is in no tree, and neither is an id the tree has never heard of: both are
// told there is nothing to inherit rather than being told the question was wrong.
func TestAuthForAnswersNothingOutsideTheTree(t *testing.T) {
	ctx := context.Background()
	a := setupAuthTree(t)

	for _, id := range []domain.DraftID{domain.DraftCommandLine, "нет-такого"} {
		got, err := a.uc.AuthFor(ctx, id)
		if err != nil {
			t.Errorf("AuthFor(%q) = %v, want no error", id, err)
		}
		if got != nil {
			t.Errorf("AuthFor(%q) = %+v, want nothing to inherit", id, got)
		}
	}
}
