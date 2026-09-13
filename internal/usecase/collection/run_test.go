package collection

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
)

// runnable is a collection with a request, a folder holding two more, and a request after the
// folder — the shape that says whether the walk goes depth first and in the tree's order.
type runnable struct {
	uc           *UseCase
	store        *fakeStore
	sender       *fakeSender
	notifier     *fakeNotifier
	collectionID string
	folderID     string
	requests     map[string]string // name → url
}

func setupRunnable(t *testing.T) *runnable {
	t.Helper()
	uc, store, sender, notifier := newTestRun()
	ctx := context.Background()

	tree, err := uc.CreateCollection(ctx, "Коллекция", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	collectionID := only(t, tree).ID

	requests := map[string]string{}
	add := func(parentID string, name, url string) string {
		t.Helper()
		tree, err := uc.CreateNode(ctx, NewNode{
			CollectionID: collectionID, ParentID: parentID, Kind: domain.NodeRequest, Name: name, Method: "GET",
		})
		if err != nil {
			t.Fatalf("CreateNode %s: %v", name, err)
		}
		node := findInTree(t, tree, name)
		if _, err := uc.SaveNode(ctx, domain.CollectionNode{
			ID: node.ID, Name: name, Method: "GET", URL: url,
			Headers: []domain.Row{
				{ID: node.ID + "-on", Name: "Accept", Value: "application/vnd.api+json", Enabled: true},
				{ID: node.ID + "-off", Name: "X-Off", Value: "нет", Enabled: false},
			},
		}); err != nil {
			t.Fatalf("SaveNode %s: %v", name, err)
		}
		requests[name] = url
		sender.reply(url, 200, 1000)
		return node.ID
	}

	add("", "Первый", "https://api.example.com/first")
	tree, err = uc.CreateNode(ctx, NewNode{
		CollectionID: collectionID, Kind: domain.NodeFolder, Name: "Папка",
	})
	if err != nil {
		t.Fatalf("CreateNode fold: %v", err)
	}
	folderID := findInTree(t, tree, "Папка").ID
	add(folderID, "Второй", "https://api.example.com/second")
	add(folderID, "Третий", "https://api.example.com/third")
	add("", "Четвёртый", "https://api.example.com/fourth")

	return &runnable{
		uc: uc, store: store, sender: sender, notifier: notifier,
		collectionID: collectionID, folderID: folderID, requests: requests,
	}
}

func findInTree(t *testing.T, tree []domain.Collection, name string) domain.CollectionNode {
	t.Helper()
	var walk func([]domain.CollectionNode) (domain.CollectionNode, bool)
	walk = func(nodes []domain.CollectionNode) (domain.CollectionNode, bool) {
		for _, node := range nodes {
			if node.Name == name {
				return node, true
			}
			if found, ok := walk(node.Items); ok {
				return found, true
			}
		}
		return domain.CollectionNode{}, false
	}
	for _, collection := range tree {
		if found, ok := walk(collection.Items); ok {
			return found
		}
	}
	t.Fatalf("no node named %s in the tree", name)
	return domain.CollectionNode{}
}

func TestRunWalksTheSubtreeDepthFirst(t *testing.T) {
	r := setupRunnable(t)

	if _, err := r.uc.Run(context.Background(), r.collectionID, ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	run := r.notifier.runFinished(t)

	want := []string{
		"https://api.example.com/first",
		"https://api.example.com/second",
		"https://api.example.com/third",
		"https://api.example.com/fourth",
	}
	got := r.sender.urls()
	if len(got) != len(want) {
		t.Fatalf("sent %d requests, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request %d = %s, want %s — the run walks the tree in the order it is drawn", i, got[i], want[i])
		}
	}

	if run.Passed != 4 || run.Failed != 0 {
		t.Errorf("run = %d passed, %d failed, want all four passed", run.Passed, run.Failed)
	}
	if len(run.Results) != 4 {
		t.Fatalf("run kept %d results, want 4", len(run.Results))
	}
	for i, result := range run.Results {
		if result.Position != int64(i) {
			t.Errorf("result %d is at position %d", i, result.Position)
		}
		if !result.OK || result.Status == nil || *result.Status != 200 || result.DurationUs != 1000 {
			t.Errorf("result %d = %+v, want a 200 with its duration", i, result)
		}
	}
	if run.DurationUs < 0 || run.FinishedAt == 0 {
		t.Errorf("run = %+v, want it closed with a time", run)
	}
}

func TestRunSendsWhatTheRequestIsMadeOf(t *testing.T) {
	r := setupRunnable(t)

	if _, err := r.uc.Run(context.Background(), r.collectionID, r.folderID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	r.notifier.runFinished(t)

	sent := r.sender.sent
	if len(sent) != 2 {
		t.Fatalf("sent %d requests, want the folder's two", len(sent))
	}
	if sent[0].Method != "GET" || sent[0].URL != r.requests["Второй"] {
		t.Errorf("request = %+v, want the saved method and address", sent[0])
	}
	// A row a person switched off travels nowhere, and the row that is on stays a pair.
	if len(sent[0].Headers) != 1 || sent[0].Headers[0].Name != "Accept" {
		t.Errorf("headers = %+v, want only the enabled row", sent[0].Headers)
	}
}

func TestRunOfAFolderTouchesOnlyItsSubtree(t *testing.T) {
	r := setupRunnable(t)

	if _, err := r.uc.Run(context.Background(), r.collectionID, r.folderID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	run := r.notifier.runFinished(t)

	if len(run.Results) != 2 || run.Passed != 2 {
		t.Errorf("run = %+v, want the folder's two requests and nothing else", run)
	}
	if run.NodeID != r.folderID {
		t.Errorf("run node = %q, want the folder it was started from", run.NodeID)
	}
}

func TestRunOfASingleRequest(t *testing.T) {
	r := setupRunnable(t)
	node := findInTree(t, mustTree(t, r), "Третий")

	if _, err := r.uc.Run(context.Background(), r.collectionID, node.ID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	run := r.notifier.runFinished(t)

	if len(run.Results) != 1 || run.Results[0].NodeID != node.ID {
		t.Errorf("run = %+v, want the one request it names", run)
	}
}

func TestRunGoesOnAfterAFailure(t *testing.T) {
	r := setupRunnable(t)
	r.sender.fail(r.requests["Второй"])

	if _, err := r.uc.Run(context.Background(), r.collectionID, ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	run := r.notifier.runFinished(t)

	// The whole point of a run: one endpoint that is down does not hide the three behind it.
	if len(run.Results) != 4 {
		t.Fatalf("run reached %d requests, want all four", len(run.Results))
	}
	if run.Passed != 3 || run.Failed != 1 {
		t.Errorf("run = %d passed, %d failed, want 3 and 1", run.Passed, run.Failed)
	}
	failed := run.Results[1]
	if failed.OK || failed.Status != nil || failed.Error != "сервер не ответил" {
		t.Errorf("failed result = %+v, want it without a status and with the reason", failed)
	}
	if run.Results[2].OK != true {
		t.Errorf("the request after a failed one = %+v, want it sent", run.Results[2])
	}
}

func TestRunStopWaitsForTheRequestInFlight(t *testing.T) {
	r := setupRunnable(t)

	// Stopping from inside a send is the harshest moment: the request is out, and the run must read
	// it to the end rather than throw its answer away.
	stopped := false
	r.sender.hook(func(RunRequest) {
		if !stopped {
			stopped = true
			r.uc.Stop()
		}
	})

	if _, err := r.uc.Run(context.Background(), r.collectionID, ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	run := r.notifier.runFinished(t)

	if len(run.Results) != 1 {
		t.Errorf("run kept %d results, want the one request it was told to finish", len(run.Results))
	}
	if len(r.sender.urls()) != 1 {
		t.Errorf("sent %+v, want nothing after the stop", r.sender.urls())
	}
}

func TestRunRefusesASecondOne(t *testing.T) {
	r := setupRunnable(t)

	// A sender that holds the run inside its first request, so the second call meets a run that is
	// still going.
	held := make(chan struct{}, 1)
	release := make(chan struct{})
	r.sender.hook(func(RunRequest) {
		select {
		case held <- struct{}{}:
		default:
		}
		<-release
	})

	if _, err := r.uc.Run(context.Background(), r.collectionID, ""); err != nil {
		t.Fatalf("Run: %v", err)
	}
	<-held

	if _, err := r.uc.Run(context.Background(), r.collectionID, r.folderID); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("a second run = %v, want ErrNotAllowed", err)
	}
	if stopped := r.uc.Stop(); !stopped {
		t.Error("Stop reported nothing to stop while a run was going")
	}

	close(release)
	r.notifier.runFinished(t)

	// With the run over, the next one is welcome again.
	r.sender.hook(nil)
	if stopped := r.uc.Stop(); stopped {
		t.Error("Stop reported a run after the run had finished")
	}
	if _, err := r.uc.Run(context.Background(), r.collectionID, r.folderID); err != nil {
		t.Errorf("a run after the first one: %v", err)
	}
	r.notifier.runFinished(t)
}

func TestRunRefusesAnEmptySubtree(t *testing.T) {
	uc, store, _, _ := newTestRun()
	ctx := context.Background()

	tree, err := uc.CreateCollection(ctx, "Пустая", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	collectionID := only(t, tree).ID

	if _, err := uc.Run(ctx, collectionID, ""); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("run of an empty collection = %v, want ErrNotAllowed", err)
	}
	// A refused run leaves nothing behind: no run row, and a next run is not told one is going.
	if len(store.runs) != 0 {
		t.Errorf("stored %d runs, want none", len(store.runs))
	}
	if _, err := uc.Run(ctx, "нет-такой", ""); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("run of a missing collection = %v, want ErrNotFound", err)
	}

	// A node of another collection is a stale selection, not an empty folder: the two are told apart
	// because the window shows them differently.
	other, err := uc.CreateCollection(ctx, "Другая", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	otherCollection, ok := findCollection(other, other[len(other)-1].ID)
	if !ok {
		t.Fatal("the second collection is not in the tree")
	}
	otherID := otherCollection.ID
	tree, err = uc.CreateNode(ctx, NewNode{CollectionID: otherID, Kind: domain.NodeRequest, Name: "Чужой"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	stranger := findInTree(t, tree, "Чужой").ID
	if _, err := uc.Run(ctx, collectionID, stranger); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("run of a node that is not in this collection = %v, want ErrNotFound", err)
	}
}

func TestLastRunReadsWhatTheRunWrote(t *testing.T) {
	r := setupRunnable(t)
	ctx := context.Background()

	last, err := r.uc.LastRun(ctx, r.collectionID, "")
	if err != nil {
		t.Fatalf("LastRun: %v", err)
	}
	if last != nil {
		t.Fatalf("LastRun = %+v, want nothing before the first run", last)
	}

	if _, err := r.uc.Run(ctx, r.collectionID, r.folderID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	r.notifier.runFinished(t)

	last, err = r.uc.LastRun(ctx, r.collectionID, r.folderID)
	if err != nil {
		t.Fatalf("LastRun: %v", err)
	}
	if last == nil || len(last.Results) != 2 || last.Passed != 2 {
		t.Fatalf("LastRun = %+v, want the folder's run with its rows", last)
	}
	if last.Results[0].NodeID != findInTree(t, mustTree(t, r), "Второй").ID {
		t.Errorf("first result = %+v, want the folder's first request", last.Results[0])
	}

	// A run of the folder is not a run of the collection: the overview of one must not draw the
	// other's summary.
	collection, err := r.uc.LastRun(ctx, r.collectionID, "")
	if err != nil {
		t.Fatalf("LastRun: %v", err)
	}
	if collection != nil {
		t.Errorf("the collection reports a run of its folder: %+v", collection)
	}
}

func TestRunPublishesEveryRequestAsItGoes(t *testing.T) {
	r := setupRunnable(t)

	if _, err := r.uc.Run(context.Background(), r.collectionID, r.folderID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	r.notifier.runFinished(t)

	progress := []RunProgress{}
	r.notifier.mu.Lock()
	for _, event := range r.notifier.events {
		if event.topic == TopicRunProgress {
			progress = append(progress, event.payload.(RunProgress))
		}
	}
	r.notifier.mu.Unlock()

	// The window draws the status bar from these, so they have to count up and end at the total —
	// a run that reported the total only at the end would show "0 из 2" until it was over.
	if len(progress) != 2 {
		t.Fatalf("published %d progress events, want one per request", len(progress))
	}
	for i, event := range progress {
		if event.Done != i+1 || event.Total != 2 {
			t.Errorf("progress %d = %d of %d, want %d of 2", i, event.Done, event.Total, i+1)
		}
	}
	if last := r.notifier.topics()[len(r.notifier.topics())-1]; last != TopicRunFinished {
		t.Errorf("the last event is %s, want the finished run", last)
	}
}

func mustTree(t *testing.T, r *runnable) []domain.Collection {
	t.Helper()
	tree, err := r.uc.Tree(context.Background())
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	return tree
}
