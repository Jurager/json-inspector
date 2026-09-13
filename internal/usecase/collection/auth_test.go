package collection

import (
	"context"
	"testing"

	"json-inspector/internal/domain"
)

// authTree is a collection with a folder in it, a request inside the folder and one beside it: the
// shape that says whether a request finds the level that answers for it.
type authTree struct {
	uc           *UseCase
	collectionID string
	folderID     string
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

	_, tree, err = uc.CreateNode(ctx, NewNode{
		CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка",
	})
	if err != nil {
		t.Fatalf("CreateNode folder: %v", err)
	}
	folderID := findInTree(t, tree, "Папка").ID

	_, tree, err = uc.CreateNode(ctx, NewNode{
		CollectionID: collectionID, ParentID: folderID, Kind: domain.NodeRequest, Name: "Внутри",
	})
	if err != nil {
		t.Fatalf("CreateNode inside: %v", err)
	}
	insideID := findInTree(t, tree, "Внутри").ID

	_, tree, err = uc.CreateNode(ctx, NewNode{
		CollectionID: collectionID, Kind: domain.NodeRequest, Name: "Рядом",
	})
	if err != nil {
		t.Fatalf("CreateNode beside: %v", err)
	}
	besideID := findInTree(t, tree, "Рядом").ID

	return authTree{uc: uc, collectionID: collectionID, folderID: folderID, insideID: insideID, besideID: besideID}
}

func bearer(token string) domain.Auth {
	return domain.Auth{Type: domain.AuthBearer, Token: token}
}

// What a level authorizes its requests with, and where a request finds it: the nearest level above
// it that answered, which is the folder for what is inside the folder and the collection for what
// is not.
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
	if got == nil || got.Token != "коллекция" {
		t.Errorf("inherited = %+v, want the collection's", got)
	}

	// A folder that answers for itself stops the walk for everything under it, and only for that.
	if _, err := a.uc.SaveAuth(ctx, a.folderID, bearer("папка")); err != nil {
		t.Fatalf("SaveAuth folder: %v", err)
	}
	if got, _ := a.uc.AuthFor(ctx, domain.DraftID(a.insideID)); got == nil || got.Token != "папка" {
		t.Errorf("inside inherited = %+v, want the folder's", got)
	}
	if got, _ := a.uc.AuthFor(ctx, domain.DraftID(a.besideID)); got == nil || got.Token != "коллекция" {
		t.Errorf("beside inherited = %+v, want the collection's", got)
	}

	// A request that answers for itself is not the folder's any more.
	if _, err := a.uc.SaveAuth(ctx, a.insideID, bearer("сам")); err != nil {
		t.Fatalf("SaveAuth request: %v", err)
	}
	if got, _ := a.uc.AuthFor(ctx, domain.DraftID(a.insideID)); got == nil || got.Token != "сам" {
		t.Errorf("own = %+v, want the request's own", got)
	}

	// «Нет» is not an answer that stops the walk: a level with nothing of its own lets the one below
	// it inherit from the one above.
	if _, err := a.uc.SaveAuth(ctx, a.folderID, domain.Auth{Type: domain.AuthNone}); err != nil {
		t.Fatalf("SaveAuth folder none: %v", err)
	}
	if got, _ := a.uc.AuthFor(ctx, domain.DraftID(a.besideID)); got == nil || got.Token != "коллекция" {
		t.Errorf("after clearing the folder = %+v, want the collection's still", got)
	}
	tree, err := a.uc.Tree(ctx)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if folder := findInTree(t, tree, "Папка"); folder.Auth != nil {
		t.Errorf("folder auth = %+v, want it stored as nothing at all", folder.Auth)
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
