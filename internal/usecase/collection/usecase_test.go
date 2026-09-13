package collection

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// fakeStore is the tree without a database: collections in their order, nodes in one flat slice,
// deleted subtrees marked rather than removed. It keeps the two things the use case relies on — the
// tree nests by parent and a node reads whole while a tree row is shallow.
type fakeStore struct {
	collections []domain.Collection
	nodes       []domain.CollectionNode
	gone        map[string]bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{gone: map[string]bool{}}
}

func (f *fakeStore) hidden(id string) bool {
	return f.gone[id]
}

// bury marks a node and everything under it, the way the schema's cascade would.
func (f *fakeStore) bury(id string) {
	if f.gone[id] {
		return
	}
	f.gone[id] = true
	for _, node := range f.nodes {
		if node.ParentID == id {
			f.bury(node.ID)
		}
	}
}

func (f *fakeStore) Collections(context.Context) ([]domain.Collection, error) {
	out := make([]domain.Collection, 0, len(f.collections))
	for _, collection := range f.collections {
		if f.hidden(collection.ID) {
			continue
		}
		collection.Items = f.nest(collection.ID)
		out = append(out, collection)
	}
	return out, nil
}

// nest builds the shallow tree the list draws: a row knows its children by shape, not by content.
func (f *fakeStore) nest(collectionID string) []domain.CollectionNode {
	var build func(parent string) []domain.CollectionNode
	build = func(parent string) []domain.CollectionNode {
		items := []domain.CollectionNode{}
		for _, node := range f.nodes {
			if node.CollectionID != collectionID || node.ParentID != parent || f.hidden(node.ID) {
				continue
			}
			row := node
			row.Items = build(row.ID)
			row.Params, row.Headers, row.Cookies, row.Body = nil, nil, nil, ""
			row.Description, row.Auth, row.URL = "", nil, ""
			items = append(items, row)
		}
		return items
	}
	return build("")
}

func (f *fakeStore) Node(_ context.Context, id string) (domain.CollectionNode, error) {
	for _, node := range f.nodes {
		if node.ID == id && !f.hidden(id) {
			node.Items = nil
			return node, nil
		}
	}
	return domain.CollectionNode{}, domain.ErrNotFound
}

func (f *fakeStore) SaveCollection(_ context.Context, collection domain.Collection) error {
	for i := range f.collections {
		if f.collections[i].ID == collection.ID {
			collection.Items = nil
			f.collections[i] = collection
			return nil
		}
	}
	collection.Items = nil
	f.collections = append(f.collections, collection)
	return nil
}

func (f *fakeStore) SaveNode(_ context.Context, node domain.CollectionNode) error {
	node.Items = nil
	for i := range f.nodes {
		if f.nodes[i].ID == node.ID {
			f.nodes[i] = node
			return nil
		}
	}
	f.nodes = append(f.nodes, node)
	return nil
}

func (f *fakeStore) DeleteCollection(_ context.Context, id string) error {
	f.bury(id)
	for _, node := range f.nodes {
		if node.CollectionID == id {
			f.bury(node.ID)
		}
	}
	f.gone[id] = true
	return nil
}

func (f *fakeStore) DeleteNode(_ context.Context, id string) error {
	f.bury(id)
	return nil
}

func (f *fakeStore) NextPosition(_ context.Context, collectionID string, parentID string) (int64, error) {
	var next int64
	for _, node := range f.nodes {
		if node.CollectionID == collectionID && node.ParentID == parentID && !f.hidden(node.ID) {
			if node.Position >= next {
				next = node.Position + 1
			}
		}
	}
	return next, nil
}

func newTestUseCase() (*UseCase, *fakeStore) {
	store := newFakeStore()
	return NewUseCase(store, platform.NewIDGen()), store
}

func only(t *testing.T, tree []domain.Collection) domain.Collection {
	t.Helper()
	if len(tree) != 1 {
		t.Fatalf("tree = %d collections, want 1", len(tree))
	}
	return tree[0]
}

func TestCreateCollectionAppends(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	first, err := uc.CreateCollection(ctx, "  Пользователи  ", " тестовые ")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	tree, err := uc.CreateCollection(ctx, "Заказы", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	if len(tree) != 2 {
		t.Fatalf("tree = %d collections, want 2", len(tree))
	}
	if tree[0].Name != "Пользователи" || tree[0].Description != "тестовые" {
		t.Errorf("first = %+v, want the name trimmed", tree[0])
	}
	if tree[1].Position != 1 {
		t.Errorf("second position = %d, want the end of the list", tree[1].Position)
	}
	if first[0].ID == tree[1].ID {
		t.Error("two collections share an id")
	}
}

func TestCreateCollectionRejectsAnEmptyName(t *testing.T) {
	uc, _ := newTestUseCase()

	if _, err := uc.CreateCollection(context.Background(), "   ", ""); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("empty name = %v, want ErrNotAllowed", err)
	}
}

func TestCreateNodeLandsAtTheEndOfItsGroup(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	tree, err := uc.CreateCollection(ctx, "Коллекция", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	id := only(t, tree).ID

	tree, err = uc.CreateNode(ctx, NewNode{CollectionID: id, Kind: domain.NodeRequest, Name: "Первый"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	folder, err := uc.CreateNode(ctx, NewNode{CollectionID: id, Kind: domain.NodeFolder, Name: "Папка"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	folderID := only(t, folder).Items[1].ID

	tree, err = uc.CreateNode(ctx, NewNode{
		CollectionID: id, ParentID: folderID, Kind: domain.NodeRequest, Name: "Внутри", Method: "post",
	})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	root := only(t, tree).Items
	if len(root) != 2 {
		t.Fatalf("root = %d nodes, want 2", len(root))
	}
	// The second request's own position is unrelated to the one inside the folder: groups count
	// separately, and a node that landed at the end of the wrong one would show it here.
	if root[0].Position != 0 || root[0].Method != "GET" {
		t.Errorf("first request = %+v, want position 0 and the default method", root[0])
	}
	if root[1].Position != 1 || root[1].Kind != domain.NodeFolder || len(root[1].Items) != 1 {
		t.Errorf("folder = %+v, want position 1 with one child", root[1])
	}
	child := root[1].Items[0]
	if child.Position != 0 || child.Method != "POST" {
		t.Errorf("child = %+v, want the first of its group and the method as typed", child)
	}
}

func TestCreateNodeRefusesARequestAsParent(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "")
	id := only(t, tree).ID
	tree, err := uc.CreateNode(ctx, NewNode{CollectionID: id, Kind: domain.NodeRequest, Name: "Запрос"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	requestID := only(t, tree).Items[0].ID

	_, err = uc.CreateNode(ctx, NewNode{CollectionID: id, ParentID: requestID, Kind: domain.NodeFolder, Name: "Внутри"})
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("a request as parent = %v, want ErrNotAllowed", err)
	}
}

func TestRenameReachesBothKinds(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "")
	collectionID := only(t, tree).ID
	tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeRequest, Name: "Запрос"})
	requestID := only(t, tree).Items[0].ID

	tree, err := uc.Rename(ctx, collectionID, "Переименована")
	if err != nil {
		t.Fatalf("Rename collection: %v", err)
	}
	if only(t, tree).Name != "Переименована" {
		t.Errorf("collection = %+v, want the new name", only(t, tree))
	}

	tree, err = uc.Rename(ctx, requestID, "  Тоже  ")
	if err != nil {
		t.Fatalf("Rename node: %v", err)
	}
	if only(t, tree).Items[0].Name != "Тоже" {
		t.Errorf("node = %+v, want the name trimmed", only(t, tree).Items[0])
	}
}

func TestDuplicateCopiesTheSubtree(t *testing.T) {
	uc, store := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "")
	collectionID := only(t, tree).ID
	tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка"})
	folderID := only(t, tree).Items[0].ID
	tree, _ = uc.CreateNode(ctx, NewNode{
		CollectionID: collectionID, ParentID: folderID, Kind: domain.NodeRequest, Name: "Внутри",
	})
	requestID := only(t, tree).Items[0].Items[0].ID

	// A request inside a folder is more than the tree row the copy starts from, and a copy that
	// forgot it would be an empty folder.
	if _, err := uc.SaveNode(ctx, domain.CollectionNode{
		ID: requestID, Name: "Внутри", Method: "PATCH", URL: "https://api.example.com/users/1",
		Headers: []domain.Row{{ID: "h1", Name: "X-Test", Value: "1", Enabled: true}},
	}); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	tree, err := uc.Duplicate(ctx, folderID)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	items := only(t, tree).Items
	if len(items) != 2 {
		t.Fatalf("root = %d nodes, want the copy beside the original", len(items))
	}
	if items[1].Name != "Папка (копия)" || items[1].Position != 1 {
		t.Errorf("copy = %+v, want it named and placed next to the original", items[1])
	}
	if len(items[1].Items) != 1 {
		t.Fatalf("copy = %+v, want its child", items[1])
	}
	copiedID := items[1].Items[0].ID
	if copiedID == requestID {
		t.Error("the copy kept the original's id")
	}
	copied, err := store.Node(ctx, copiedID)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if copied.CollectionID != collectionID || copied.ParentID != items[1].ID {
		t.Errorf("copied child = %+v, want it inside the copy", copied)
	}
	if copied.URL != "https://api.example.com/users/1" || len(copied.Headers) != 1 {
		t.Errorf("copied child = %+v, want the request's own fields", copied)
	}
	if copied.Method != "PATCH" {
		t.Errorf("copied method = %q, want the original's", copied.Method)
	}
}

func TestDuplicateCopiesACollection(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "описание")
	collectionID := only(t, tree).ID
	tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeRequest, Name: "Первый"})
	tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка"})

	tree, err := uc.Duplicate(ctx, collectionID)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if len(tree) != 2 {
		t.Fatalf("tree = %d collections, want 2", len(tree))
	}
	copied := tree[1]
	if copied.Name != "Коллекция (копия)" || copied.Description != "описание" || copied.Position != 1 {
		t.Errorf("copy = %+v, want it named and placed after the original", copied)
	}
	if len(copied.Items) != 2 || copied.Items[1].Kind != domain.NodeFolder {
		t.Fatalf("copy items = %+v, want both nodes in order", copied.Items)
	}
	// Every node has to move into the new collection: a row left behind would be visible in both.
	for _, node := range copied.Items {
		if node.CollectionID != copied.ID {
			t.Errorf("node %s still lives in %s", node.ID, node.CollectionID)
		}
	}
	if copied.Items[0].ID == tree[0].Items[0].ID {
		t.Error("the copy kept the original's node id")
	}
}

func TestDuplicateClipsTheNameAtTheCeiling(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	long := ""
	for len([]rune(long)) < maxNameLength {
		long += "я"
	}
	tree, err := uc.CreateCollection(ctx, long, "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	tree, err = uc.Duplicate(ctx, only(t, tree).ID)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if got := len([]rune(tree[1].Name)); got > maxNameLength {
		t.Errorf("name is %d runes, want at most %d", got, maxNameLength)
	}
}

func TestDeleteTakesTheWholeSubtree(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "")
	collectionID := only(t, tree).ID
	tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка"})
	folderID := only(t, tree).Items[0].ID
	uc.CreateNode(ctx, NewNode{CollectionID: collectionID, ParentID: folderID, Kind: domain.NodeRequest, Name: "Внутри"})

	tree, err := uc.Delete(ctx, folderID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(only(t, tree).Items) != 0 {
		t.Errorf("items = %+v, want the folder and its child gone", only(t, tree).Items)
	}

	tree, err = uc.Delete(ctx, collectionID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(tree) != 0 {
		t.Errorf("tree = %+v, want nothing left", tree)
	}
}

func TestSaveNodeKeepsWhatTheTreeOwns(t *testing.T) {
	uc, store := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "")
	collectionID := only(t, tree).ID
	tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка"})
	folderID := only(t, tree).Items[0].ID
	tree, _ = uc.CreateNode(ctx, NewNode{
		CollectionID: collectionID, ParentID: folderID, Kind: domain.NodeRequest, Name: "Запрос",
	})
	requestID := only(t, tree).Items[0].Items[0].ID

	before, err := store.Node(ctx, requestID)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}

	// The card knows nothing about where the node lives, so it sends what it has and the rest must
	// survive: a save that moved a request to the root would be a lost tree.
	if _, err := uc.SaveNode(ctx, domain.CollectionNode{
		ID: requestID, Name: "  Переименован  ", Method: "delete", URL: "https://api.example.com/users",
		Description: "  про пользователей  ", ParentID: "", CollectionID: "",
	}); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	after, err := store.Node(ctx, requestID)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if after.Name != "Переименован" || after.Description != "про пользователей" {
		t.Errorf("node = %+v, want the edited name trimmed", after)
	}
	if after.Method != "DELETE" {
		t.Errorf("method = %q, want it upper-cased", after.Method)
	}
	if after.ParentID != before.ParentID || after.Position != before.Position ||
		after.CollectionID != before.CollectionID || after.CreatedAt != before.CreatedAt {
		t.Errorf("node = %+v, want the place it had (%+v)", after, before)
	}
}

func TestSaveNodeRejectsAnEmptyName(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "")
	collectionID := only(t, tree).ID
	tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeRequest, Name: "Запрос"})
	requestID := only(t, tree).Items[0].ID

	_, err := uc.SaveNode(ctx, domain.CollectionNode{ID: requestID, Name: "   "})
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("empty name = %v, want ErrNotAllowed", err)
	}
}

func TestNodeInAMissingTree(t *testing.T) {
	uc, _ := newTestUseCase()

	if _, err := uc.Node(context.Background(), "нет-такого"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing node = %v, want ErrNotFound", err)
	}
}
