package sqlite

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
)

// folder and request build the two node kinds the way the use case does, so the fixtures read like
// a real tree instead of a list of struct literals.
func folder(id, collectionID, parentID string, position int64, name string) domain.CollectionNode {
	return domain.CollectionNode{
		ID: id, CollectionID: collectionID, ParentID: parentID,
		Kind: domain.NodeFolder, Name: name, Position: position,
	}
}

func request(id, collectionID, parentID string, position int64, name, method, url string) domain.CollectionNode {
	return domain.CollectionNode{
		ID: id, CollectionID: collectionID, ParentID: parentID,
		Kind: domain.NodeRequest, Name: name, Position: position,
		Method: method, URL: url,
		Params:  []domain.Row{{ID: id + "-p", Name: "page", Value: "2", Enabled: true}},
		Headers: []domain.Row{{ID: id + "-h", Name: "Accept", Value: "application/vnd.api+json", Enabled: true}},
		Body:    `{"data": {"type": "users"}}`,
		Cookies: []domain.CookieRow{{ID: id + "-c", Name: "session", Value: "abc", Path: "/", HTTPOnly: true}},
	}
}

// seedTree writes one collection with a folder inside it, a request inside the folder and a request
// at the root, which is every shape the tree has to keep straight.
func seedTree(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()

	bearer := &domain.Auth{Type: domain.AuthBearer, Token: "{{token}}"}
	if err := store.SaveCollection(ctx, domain.Collection{
		ID: "col-1", Name: "Пользователи", Description: "тестовые", Position: 0, Auth: bearer,
	}); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}
	for _, node := range []domain.CollectionNode{
		folder("f-1", "col-1", "", 0, "Админ"),
		request("r-1", "col-1", "f-1", 0, "Список", "GET", "https://api.example.com/users"),
		request("r-2", "col-1", "", 1, "Один", "PATCH", "https://api.example.com/users/1"),
	} {
		if err := store.SaveNode(ctx, node); err != nil {
			t.Fatalf("SaveNode %s: %v", node.ID, err)
		}
	}
}

func TestCollectionsReadsTheTreeNested(t *testing.T) {
	store := newMigratedStore(t)
	seedTree(t, store)

	tree, err := store.Collections(context.Background())
	if err != nil {
		t.Fatalf("Collections: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("tree = %d collections, want 1", len(tree))
	}
	collection := tree[0]
	if collection.Name != "Пользователи" || collection.Description != "тестовые" {
		t.Errorf("collection = %+v, want the saved one", collection)
	}
	if collection.Auth == nil || collection.Auth.Token != "{{token}}" {
		t.Errorf("collection auth = %+v, want what everything inside inherits", collection.Auth)
	}
	if len(collection.Items) != 2 {
		t.Fatalf("root = %d nodes, want 2", len(collection.Items))
	}
	// The tree is drawn in this order, so it is the order the rows have to arrive in.
	if collection.Items[0].ID != "f-1" || collection.Items[1].ID != "r-2" {
		t.Errorf("root = %+v, want the folder and the request in position order", collection.Items)
	}
	if len(collection.Items[0].Items) != 1 || collection.Items[0].Items[0].ID != "r-1" {
		t.Fatalf("folder = %+v, want its request inside it", collection.Items[0])
	}
	// A tree row carries the method because that is what it draws, and nothing heavier: two hundred
	// bodies is not what a list loads.
	row := collection.Items[1]
	if row.Method != "PATCH" {
		t.Errorf("method = %q, want the row to carry it", row.Method)
	}
	if row.URL != "" || row.Body != "" || len(row.Headers) != 0 || len(row.Params) != 0 {
		t.Errorf("row = %+v, want a tree row with no request payload", row)
	}
}

func TestCollectionsKeepsTheirOwnOrder(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	for _, c := range []domain.Collection{
		{ID: "col-b", Name: "Вторая", Position: 1},
		{ID: "col-a", Name: "Первая", Position: 0},
	} {
		if err := store.SaveCollection(ctx, c); err != nil {
			t.Fatalf("SaveCollection: %v", err)
		}
	}

	tree, err := store.Collections(ctx)
	if err != nil {
		t.Fatalf("Collections: %v", err)
	}
	if len(tree) != 2 || tree[0].ID != "col-a" || tree[1].ID != "col-b" {
		t.Errorf("tree = %+v, want the position order, not the insert order", tree)
	}
}

func TestCollectionsDoNotMixNodes(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTree(t, store)

	if err := store.SaveCollection(ctx, domain.Collection{ID: "col-2", Name: "Заказы", Position: 1}); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}
	if err := store.SaveNode(ctx, request("r-3", "col-2", "", 0, "Заказы", "GET", "https://api.example.com/orders")); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	tree, err := store.Collections(ctx)
	if err != nil {
		t.Fatalf("Collections: %v", err)
	}
	if len(tree[1].Items) != 1 || tree[1].Items[0].ID != "r-3" {
		t.Errorf("second collection = %+v, want only its own node", tree[1].Items)
	}
	if len(tree[0].Items) != 2 {
		t.Errorf("first collection = %+v, want only its own nodes", tree[0].Items)
	}
}

func TestNodeRoundTrip(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTree(t, store)

	node, err := store.Node(ctx, "r-1")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if node.Name != "Список" || node.Method != "GET" || node.URL != "https://api.example.com/users" {
		t.Errorf("node = %+v, want the saved request", node)
	}
	if node.ParentID != "f-1" || node.CollectionID != "col-1" || node.Position != 0 {
		t.Errorf("node = %+v, want its place in the tree", node)
	}
	if len(node.Params) != 1 || node.Params[0].Value != "2" {
		t.Errorf("params = %+v, want the row with its id", node.Params)
	}
	if len(node.Headers) != 1 || node.Headers[0].Name != "Accept" {
		t.Errorf("headers = %+v", node.Headers)
	}
	if node.Body != `{"data": {"type": "users"}}` {
		t.Errorf("body = %q", node.Body)
	}
	if len(node.Cookies) != 1 || node.Cookies[0].HTTPOnly != true || node.Cookies[0].ID != "r-1-c" {
		t.Errorf("cookies = %+v, want the row with its attributes", node.Cookies)
	}
	if node.Auth != nil {
		t.Errorf("auth = %+v, want nil for a node that inherits", node.Auth)
	}
}

func TestNodeAuthRemembersExplicitNothing(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTree(t, store)

	// An empty auth is an answer, not an absence: without it a collection could not say "no auth for
	// anything in here", which is the whole point of the NULL column.
	none := &domain.Auth{Type: domain.AuthNone}
	node := request("r-4", "col-1", "", 2, "Без авторизации", "GET", "https://api.example.com/open")
	node.Auth = none
	if err := store.SaveNode(ctx, node); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	read, err := store.Node(ctx, "r-4")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if read.Auth == nil || read.Auth.Type != domain.AuthNone {
		t.Errorf("auth = %+v, want the explicit none back", read.Auth)
	}
}

func TestNodeInAMissingRow(t *testing.T) {
	store := newMigratedStore(t)

	if _, err := store.Node(context.Background(), "нет-такого"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing node = %v, want ErrNotFound", err)
	}
}

func TestSaveNodeUpdatesInPlace(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTree(t, store)

	before, err := store.Node(ctx, "r-2")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	before.Name = "Один пользователь"
	before.Headers = []domain.Row{{ID: "h", Name: "If-None-Match", Value: `"v1"`, Enabled: true}}
	if err := store.SaveNode(ctx, before); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	after, err := store.Node(ctx, "r-2")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if after.Name != "Один пользователь" || len(after.Headers) != 1 || after.Headers[0].Name != "If-None-Match" {
		t.Errorf("node = %+v, want the edit", after)
	}
	if after.CreatedAt != before.CreatedAt || after.Position != before.Position {
		t.Errorf("node = %+v, want created_at and position untouched by an edit", after)
	}

	tree, err := store.Collections(ctx)
	if err != nil {
		t.Fatalf("Collections: %v", err)
	}
	if tree[0].Items[1].Name != "Один пользователь" {
		t.Errorf("tree = %+v, want the new name in the row", tree[0].Items[1])
	}
}

func TestNextPositionCountsPerGroup(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTree(t, store)

	next, err := store.NextPosition(ctx, "col-1", "")
	if err != nil {
		t.Fatalf("NextPosition: %v", err)
	}
	if next != 2 {
		t.Errorf("next at the root = %d, want one past the last sibling", next)
	}

	next, err = store.NextPosition(ctx, "col-1", "f-1")
	if err != nil {
		t.Fatalf("NextPosition: %v", err)
	}
	if next != 1 {
		t.Errorf("next inside the folder = %d, want its own count", next)
	}

	next, err = store.NextPosition(ctx, "col-2", "")
	if err != nil {
		t.Fatalf("NextPosition: %v", err)
	}
	if next != 0 {
		t.Errorf("next in an empty collection = %d, want 0", next)
	}
}

func TestDeleteTakesTheSubtreeWithIt(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTree(t, store)

	if err := store.DeleteNode(ctx, "f-1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if _, err := store.Node(ctx, "r-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a request inside a deleted folder = %v, want it gone too", err)
	}

	node, err := store.Node(ctx, "r-2")
	if err != nil {
		t.Fatalf("Node: %v, want a sibling of the deleted folder", err)
	}
	if node.Name != "Один" {
		t.Errorf("node = %+v, want the root request untouched", node)
	}

	if err := store.DeleteCollection(ctx, "col-1"); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}
	if _, err := store.Node(ctx, "r-2"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a node of a deleted collection = %v, want it gone too", err)
	}
	tree, err := store.Collections(ctx)
	if err != nil {
		t.Fatalf("Collections: %v", err)
	}
	if len(tree) != 0 {
		t.Errorf("tree = %+v, want nothing left", tree)
	}
}

func TestNodesSurviveACollectionRename(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTree(t, store)

	tree, err := store.Collections(ctx)
	if err != nil {
		t.Fatalf("Collections: %v", err)
	}
	collection := tree[0]
	collection.Name = "Пользователи API"
	if err := store.SaveCollection(ctx, collection); err != nil {
		t.Fatalf("SaveCollection: %v", err)
	}

	tree, err = store.Collections(ctx)
	if err != nil {
		t.Fatalf("Collections: %v", err)
	}
	if tree[0].Name != "Пользователи API" {
		t.Errorf("collection = %+v, want the new name", tree[0])
	}
	if tree[0].Auth == nil {
		t.Error("the rename dropped the collection's auth")
	}
	if len(tree[0].Items) != 2 || len(tree[0].Items[0].Items) != 1 {
		t.Errorf("tree = %+v, want the nodes where they were", tree[0].Items)
	}
}
