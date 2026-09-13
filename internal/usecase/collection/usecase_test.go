package collection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// fakeStore is the tree without a database: collections in their order, nodes in one flat slice,
// deleted subtrees marked rather than removed. It keeps the two things the use case relies on — the
// tree nests by parent and a node reads whole while a tree row is shallow.
//
// A run writes to it from its own goroutine while a test reads, so the mutex is the fake standing in
// for the database's own serialisation.
type fakeStore struct {
	mu          sync.Mutex
	collections []domain.Collection
	nodes       []domain.CollectionNode
	runs        []domain.CollectionRun
	results     map[string][]domain.CollectionRunResult
	gone        map[string]bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{gone: map[string]bool{}, results: map[string][]domain.CollectionRunResult{}}
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
	f.mu.Lock()
	defer f.mu.Unlock()

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
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, node := range f.nodes {
		if node.ID == id && !f.hidden(id) {
			node.Items = nil
			return node, nil
		}
	}
	return domain.CollectionNode{}, domain.ErrNotFound
}

func (f *fakeStore) SaveCollection(_ context.Context, collection domain.Collection) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	collection.Items = nil
	for i := range f.collections {
		if f.collections[i].ID == collection.ID {
			f.collections[i] = collection
			return nil
		}
	}
	f.collections = append(f.collections, collection)
	return nil
}

func (f *fakeStore) SaveNode(_ context.Context, node domain.CollectionNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()

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
	f.mu.Lock()
	defer f.mu.Unlock()

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
	f.mu.Lock()
	defer f.mu.Unlock()

	f.bury(id)
	return nil
}

func (f *fakeStore) NextPosition(_ context.Context, collectionID string, parentID string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

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

func (f *fakeStore) SaveRun(_ context.Context, run domain.CollectionRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	run.Results = nil
	for i := range f.runs {
		if f.runs[i].ID == run.ID {
			f.runs[i] = run
			return nil
		}
	}
	f.runs = append(f.runs, run)
	return nil
}

func (f *fakeStore) AppendRunResult(_ context.Context, runID string, result domain.CollectionRunResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.results[runID] = append(f.results[runID], result)
	return nil
}

func (f *fakeStore) LastRun(_ context.Context, collectionID string, nodeID string) (domain.CollectionRun, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i := len(f.runs) - 1; i >= 0; i-- {
		run := f.runs[i]
		if run.CollectionID == collectionID && run.NodeID == nodeID {
			run.Results = append([]domain.CollectionRunResult{}, f.results[run.ID]...)
			return run, true, nil
		}
	}
	return domain.CollectionRun{}, false, nil
}

// fakeSender answers with what the test set up for a URL. It is the only thing the run knows about
// the network, which is what lets a test decide what a server did.
type fakeSender struct {
	mu      sync.Mutex
	sent    []RunRequest
	answers map[string]answer
	// onSend runs inside the send, which is where a test can look at a run while it is going.
	onSend func(RunRequest)
}

type answer struct {
	status      int
	durationUs  int64
	transportEr string
}

func newFakeSender() *fakeSender {
	return &fakeSender{answers: map[string]answer{}}
}

func (f *fakeSender) reply(url string, status int, durationUs int64) *fakeSender {
	f.answers[url] = answer{status: status, durationUs: durationUs}
	return f
}

func (f *fakeSender) fail(url string) *fakeSender {
	f.answers[url] = answer{transportEr: "сервер не ответил"}
	return f
}

// hook sets what happens inside a send, which is the only moment a test can look at a run while it
// is going.
func (f *fakeSender) hook(onSend func(RunRequest)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.onSend = onSend
}

func (f *fakeSender) Send(_ context.Context, req RunRequest) (domain.Record, error) {
	f.mu.Lock()
	onSend := f.onSend
	f.sent = append(f.sent, req)
	answer, ok := f.answers[req.URL]
	f.mu.Unlock()

	// Outside the lock: a hook that stops the run is a caller of this feature, not of this fake.
	if onSend != nil {
		onSend(req)
	}

	if !ok {
		return domain.Record{}, fmt.Errorf("нет ответа для %s", req.URL)
	}
	return domain.Record{RecordSummary: domain.RecordSummary{
		Status:     answer.status,
		DurationUs: answer.durationUs,
		Error:      answer.transportEr,
	}}, nil
}

func (f *fakeSender) urls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []string{}
	for _, req := range f.sent {
		out = append(out, req.URL)
	}
	return out
}

// fakeNotifier keeps what was published and lets a test wait for the end of a run instead of
// sleeping: the run writes from its own goroutine.
type fakeNotifier struct {
	mu     sync.Mutex
	events []struct {
		topic   string
		payload any
	}
	finished chan struct{}
}

func newFakeNotifier() *fakeNotifier {
	return &fakeNotifier{finished: make(chan struct{}, 1)}
}

func (n *fakeNotifier) Publish(topic string, payload any) {
	n.mu.Lock()
	n.events = append(n.events, struct {
		topic   string
		payload any
	}{topic, payload})
	n.mu.Unlock()

	if topic == TopicRunFinished {
		select {
		case n.finished <- struct{}{}:
		default:
		}
	}
}

func (n *fakeNotifier) topics() []string {
	n.mu.Lock()
	defer n.mu.Unlock()

	out := []string{}
	for _, event := range n.events {
		out = append(out, event.topic)
	}
	return out
}

func (n *fakeNotifier) runFinished(t *testing.T) domain.CollectionRun {
	t.Helper()
	select {
	case <-n.finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the run never finished")
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	for _, event := range n.events {
		if event.topic == TopicRunFinished {
			return event.payload.(domain.CollectionRun)
		}
	}
	t.Fatal("no finished run was published")
	return domain.CollectionRun{}
}

func newTestUseCase() (*UseCase, *fakeStore) {
	uc, store, _, _ := newTestRun()
	return uc, store
}

// newTestRun is the use case with all three of its dependencies, for the tests that also look at
// what was sent and what was published.
func newTestRun() (*UseCase, *fakeStore, *fakeSender, *fakeNotifier) {
	store := newFakeStore()
	sender := newFakeSender()
	notifier := newFakeNotifier()
	return NewUseCase(store, sender, notifier, platform.NewIDGen()), store, sender, notifier
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

	_, tree, err = uc.CreateNode(ctx, NewNode{CollectionID: id, Kind: domain.NodeRequest, Name: "Первый"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	_, folder, err := uc.CreateNode(ctx, NewNode{CollectionID: id, Kind: domain.NodeFolder, Name: "Папка"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	folderID := only(t, folder).Items[1].ID

	_, tree, err = uc.CreateNode(ctx, NewNode{
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
	_, tree, err := uc.CreateNode(ctx, NewNode{CollectionID: id, Kind: domain.NodeRequest, Name: "Запрос"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	requestID := only(t, tree).Items[0].ID

	_, _, err = uc.CreateNode(ctx, NewNode{CollectionID: id, ParentID: requestID, Kind: domain.NodeFolder, Name: "Внутри"})
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("a request as parent = %v, want ErrNotAllowed", err)
	}
}

// A request saved from a composer is written whole: the row that comes back is the one that
// appeared, with everything the request already had, so nothing depends on finding it again by name.
func TestCreateNodeTakesAWholeRequest(t *testing.T) {
	uc, store := newTestUseCase()
	ctx := context.Background()

	tree, err := uc.CreateCollection(ctx, "Коллекция", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	collectionID := only(t, tree).ID

	created, tree, err := uc.CreateNode(ctx, NewNode{
		CollectionID: collectionID,
		Kind:         domain.NodeRequest,
		Name:         "Сохранённый",
		Method:       "patch",
		URL:          "https://api.example.com/users/1?page=2",
		Headers:      []domain.Row{{ID: "h1", Name: "Accept", Value: "application/vnd.api+json", Enabled: true}},
		Body:         `{"data": 1}`,
		Auth:         &domain.Auth{Type: domain.AuthBearer, Token: "{{token}}"},
	})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if created.ID == "" || created.Name != "Сохранённый" {
		t.Fatalf("created = %+v, want the row that appeared", created)
	}
	if len(only(t, tree).Items) != 1 || only(t, tree).Items[0].ID != created.ID {
		t.Errorf("tree = %+v, want the created row in it", only(t, tree).Items)
	}

	stored, err := store.Node(ctx, created.ID)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if stored.Method != "PATCH" || stored.URL != "https://api.example.com/users/1?page=2" {
		t.Errorf("stored = %+v, want the request it was given", stored)
	}
	if len(stored.Headers) != 1 || stored.Body != `{"data": 1}` {
		t.Errorf("stored = %+v, want its headers and body", stored)
	}
	if stored.Auth == nil || stored.Auth.Token != "{{token}}" {
		t.Errorf("auth = %+v, want the choice the composer made", stored.Auth)
	}
}

func TestRenameReachesBothKinds(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "")
	collectionID := only(t, tree).ID
	_, tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeRequest, Name: "Запрос"})
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
	_, tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка"})
	folderID := only(t, tree).Items[0].ID
	_, tree, _ = uc.CreateNode(ctx, NewNode{
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

// A copy of a request is the request, not its name: the tree row a duplicate starts from carries the
// method and nothing else, and taking that for the content is how a copy comes out empty.
func TestDuplicateCopiesTheRequestItself(t *testing.T) {
	uc, store := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "")
	collectionID := only(t, tree).ID
	_, tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeRequest, Name: "Запрос"})
	requestID := only(t, tree).Items[0].ID

	bearer := &domain.Auth{Type: domain.AuthBearer, Token: "{{token}}"}
	if _, err := uc.SaveNode(ctx, domain.CollectionNode{
		ID: requestID, Name: "Запрос", Method: "PATCH", URL: "https://api.example.com/users/1?page=2",
		Description: "про пользователя",
		Params:      []domain.Row{{ID: "p1", Name: "page", Value: "2", Enabled: true}},
		Headers:     []domain.Row{{ID: "h1", Name: "Accept", Value: "application/vnd.api+json", Enabled: true}},
		Body:        `{"data": {"type": "users"}}`,
		Cookies:     []domain.CookieRow{{ID: "c1", Name: "session", Value: "abc", Path: "/", HTTPOnly: true}},
		Auth:        bearer,
	}); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	tree, err := uc.Duplicate(ctx, requestID)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	copiedID := only(t, tree).Items[1].ID
	copied, err := store.Node(ctx, copiedID)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}

	if copied.Name != "Запрос (копия)" || copied.Position != 1 {
		t.Errorf("copy = %+v, want it named and placed beside the original", copied)
	}
	if copied.URL != "https://api.example.com/users/1?page=2" || copied.Method != "PATCH" ||
		copied.Body != `{"data": {"type": "users"}}` {
		t.Errorf("copy = %+v, want the request it was copied from", copied)
	}
	if len(copied.Params) != 1 || len(copied.Headers) != 1 || len(copied.Cookies) != 1 {
		t.Errorf("copy = %+v, want its rows", copied)
	}
	if copied.Description != "про пользователя" {
		t.Errorf("description = %q, want it carried over", copied.Description)
	}
	if copied.Auth == nil || copied.Auth.Token != "{{token}}" {
		t.Errorf("auth = %+v, want it carried over", copied.Auth)
	}
	// The copy is a second thing: its own rows, so editing one does not edit the other.
	if copied.Params[0].ID == "p1" || copied.Headers[0].ID == "h1" {
		t.Errorf("copy = %+v, want rows of its own", copied)
	}
}

func TestDuplicateCopiesACollection(t *testing.T) {
	uc, store := newTestUseCase()
	ctx := context.Background()

	tree, _ := uc.CreateCollection(ctx, "Коллекция", "описание")
	collectionID := only(t, tree).ID
	_, tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeRequest, Name: "Первый"})
	_, tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка"})
	// The root of a collection is a tree row too, and a copy that took the row for the content would
	// lose the request behind it — the same trap the node duplicate had.
	if _, err := uc.SaveNode(ctx, domain.CollectionNode{
		ID: only(t, tree).Items[0].ID, Name: "Первый", Method: "GET", URL: "https://api.example.com/first",
	}); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

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
	copiedFirst, err := store.Node(ctx, copied.Items[0].ID)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if copiedFirst.Name != "Первый" || copiedFirst.URL != "https://api.example.com/first" {
		t.Errorf("copied root = %+v, want the request it was copied from", copiedFirst)
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
	_, tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка"})
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
	_, tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка"})
	folderID := only(t, tree).Items[0].ID
	_, tree, _ = uc.CreateNode(ctx, NewNode{
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
	_, tree, _ = uc.CreateNode(ctx, NewNode{CollectionID: collectionID, Kind: domain.NodeRequest, Name: "Запрос"})
	requestID := only(t, tree).Items[0].ID

	_, err := uc.SaveNode(ctx, domain.CollectionNode{ID: requestID, Name: "   "})
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("empty name = %v, want ErrNotAllowed", err)
	}
}

// A collection made elsewhere becomes a collection here: its name, its order, and its requests —
// with ids minted for everything a file does not write.
func TestImportTakesAWholeCollection(t *testing.T) {
	uc, store := newTestUseCase()
	ctx := context.Background()

	tree, err := uc.Import(ctx, domain.Collection{
		Name: "  Импортированная  ",
		Items: []domain.CollectionNode{
			{
				Kind: domain.NodeFolder, Name: "Папка", Items: []domain.CollectionNode{
					{Kind: domain.NodeRequest, Name: "Внутри", Method: "post", URL: "https://api.example.com/inside",
						Headers: []domain.Row{{Name: "Accept", Value: "application/json", Enabled: true}}},
				},
			},
			{
				Kind: domain.NodeRequest, Name: "Снаружи", Method: "GET", URL: "https://api.example.com/out?page=2",
				Params: []domain.Row{{Name: "page", Value: "2", Enabled: true}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	collection := only(t, tree)
	if collection.Name != "Импортированная" || collection.Position != 0 {
		t.Errorf("collection = %+v, want the file's name, trimmed", collection)
	}
	if len(collection.Items) != 2 || collection.Items[0].Items[0].Name != "Внутри" {
		t.Fatalf("items = %+v, want the file's tree", collection.Items)
	}
	for _, node := range []domain.CollectionNode{collection.Items[0], collection.Items[1], collection.Items[0].Items[0]} {
		if node.ID == "" || node.CollectionID != collection.ID {
			t.Errorf("node = %+v, want an id and the collection it was imported into", node)
		}
	}
	if collection.Items[0].Items[0].ParentID != collection.Items[0].ID {
		t.Error("the row inside the folder does not name it as its parent")
	}

	// What the tree carries is a row; what was stored is the request.
	stored, err := store.Node(ctx, collection.Items[1].ID)
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if stored.URL != "https://api.example.com/out?page=2" || stored.Method != "GET" {
		t.Errorf("stored = %+v, want the request the file had", stored)
	}
	if len(stored.Params) != 1 || stored.Params[0].ID == "" {
		t.Errorf("params = %+v, want the row with an id the window can address", stored.Params)
	}
	if stored.Position != 1 || stored.Kind != domain.NodeRequest {
		t.Errorf("stored = %+v, want its place in the imported order", stored)
	}
}

// An export writes down what a tree row does not carry: Full reads the nodes whole, and a request
// on its own is the collection an export of one request makes.
func TestFullReadsTheRequestsWhole(t *testing.T) {
	uc, _ := newTestUseCase()
	ctx := context.Background()

	tree, err := uc.CreateCollection(ctx, "Коллекция", "описание")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	collectionID := only(t, tree).ID
	_, tree, err = uc.CreateNode(ctx, NewNode{
		CollectionID: collectionID, Kind: domain.NodeRequest, Name: "Запрос", Method: "GET",
		URL: "https://api.example.com/users", Body: `{"a": 1}`,
		Headers: []domain.Row{{Name: "Accept", Value: "application/vnd.api+json", Enabled: true}},
	})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	requestID := only(t, tree).Items[0].ID

	full, err := uc.Full(ctx, collectionID)
	if err != nil {
		t.Fatalf("Full: %v", err)
	}
	if full.Name != "Коллекция" || full.Description != "описание" || len(full.Items) != 1 {
		t.Fatalf("full = %+v, want the collection and its request", full)
	}
	if full.Items[0].Body != `{"a": 1}` || len(full.Items[0].Headers) != 1 {
		t.Errorf("request = %+v, want it read whole", full.Items[0])
	}
	if full.Items[0].Kind != domain.NodeRequest {
		t.Errorf("kind = %q, want a request", full.Items[0].Kind)
	}

	single, err := uc.Full(ctx, requestID)
	if err != nil {
		t.Fatalf("Full: %v", err)
	}
	if single.Name != "Запрос" || len(single.Items) != 1 || single.Items[0].URL != "https://api.example.com/users" {
		t.Errorf("single = %+v, want a collection of the one request", single)
	}
}

func TestNodeInAMissingTree(t *testing.T) {
	uc, _ := newTestUseCase()

	if _, err := uc.Node(context.Background(), "нет-такого"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing node = %v, want ErrNotFound", err)
	}
}
