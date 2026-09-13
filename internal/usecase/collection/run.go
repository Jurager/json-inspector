package collection

import (
	"context"
	"fmt"
	"strings"
	"time"

	"json-inspector/internal/domain"
)

// Event topics. A run reports itself twice: once per request that finished, which is both the
// progress and the row, and once at the end.
const (
	TopicRunProgress = "collection:run-progress"
	TopicRunFinished = "collection:run-finished"
)

// RunProgress is one request of a run finishing. The counters are here and not in the window: the
// window that opened the collections view while a run was going would otherwise count from the
// middle, and the request that just finished is a row it can draw at once.
type RunProgress struct {
	RunID  string                     `json:"runId"`
	Done   int                        `json:"done"`
	Total  int                        `json:"total"`
	Result domain.CollectionRunResult `json:"result"`
}

// Run executes everything under a node, one request after another, and answers with the id of the
// run at once: a collection of fifty requests is a minute of waiting, and the window is told how it
// is going rather than holding a promise.
//
// A folder is run as itself — a run of the whole collection is a run of the collection, and the two
// are told apart by what the run was started from.
func (u *UseCase) Run(ctx context.Context, collectionID string, nodeID string) (string, error) {
	// One run at a time: two of them would interleave their rows into the same overview, and the
	// button that started the second one says "Остановить" rather than "Запустить".
	if !u.running.CompareAndSwap(false, true) {
		return "", fmt.Errorf("прогон уже идёт: %w", domain.ErrNotAllowed)
	}

	tree, err := u.store.Collections(ctx)
	if err != nil {
		u.running.Store(false)
		return "", err
	}

	collection, ok := findCollection(tree, collectionID)
	if !ok {
		u.running.Store(false)
		return "", fmt.Errorf("коллекция %s: %w", collectionID, domain.ErrNotFound)
	}

	requests, err := requestsUnder(collection, nodeID)
	if err != nil {
		u.running.Store(false)
		return "", err
	}
	if len(requests) == 0 {
		// Nothing to send is a thing the window asks by mistake — an empty folder, a collection not
		// filled yet — and a run that reports "0 из 0" reads as a failure of the app.
		u.running.Store(false)
		return "", fmt.Errorf("в этом узле нет запросов: %w", domain.ErrNotAllowed)
	}

	run := domain.CollectionRun{
		ID:           u.ids(),
		CollectionID: collectionID,
		NodeID:       nodeID,
		StartedAt:    time.Now().UnixMilli(),
		Results:      []domain.CollectionRunResult{},
	}
	// The run is written before anything is sent: its results hang off it by a foreign key, and a
	// run the window was told about must exist even if the app is closed mid-flight.
	if err := u.store.SaveRun(ctx, run); err != nil {
		u.running.Store(false)
		return "", err
	}

	u.stopped.Store(false)
	go u.execute(run, requests)
	return run.ID, nil
}

// Stop asks the run to stop. The request already in flight is read to the end: it is on its way,
// and its answer is a row the run is owed.
func (u *UseCase) Stop() bool {
	if !u.running.Load() {
		return false
	}
	u.stopped.Store(true)
	return true
}

// LastRun is what the overview draws when it opens. Absent is an answer — nothing has been run here
// yet — and it is nil rather than an error, because a collection nobody has run is not a problem.
func (u *UseCase) LastRun(ctx context.Context, collectionID string, nodeID string) (*domain.CollectionRun, error) {
	run, ok, err := u.store.LastRun(ctx, collectionID, nodeID)
	if err != nil || !ok {
		return nil, err
	}
	return &run, nil
}

// execute walks the requests and sends them one at a time. It runs on a context of its own: the
// caller's ends when the frontend call returns, and a run outlives the call that started it.
func (u *UseCase) execute(run domain.CollectionRun, requests []runnable) {
	ctx := context.Background()
	defer u.running.Store(false)

	started := time.Now()
	for position, item := range requests {
		if u.stopped.Load() {
			break
		}

		result := u.attempt(ctx, item, int64(position), run.ID)
		// The row goes in before it is counted: a run whose results cannot be written is over, and
		// a summary that counts a row the database does not have is a summary that lies.
		if err := u.store.AppendRunResult(ctx, run.ID, result); err != nil {
			break
		}
		run.Results = append(run.Results, result)
		switch {
		case result.Skipped:
			// Neither a pass nor a failure: a script kept this request from going out, and counting
			// it either way would say something that did not happen.
		case result.OK:
			run.Passed++
		default:
			run.Failed++
		}
		u.notifier.Publish(TopicRunProgress, RunProgress{
			RunID:  run.ID,
			Done:   len(run.Results),
			Total:  len(requests),
			Result: result,
		})
	}

	run.DurationUs = time.Since(started).Microseconds()
	run.FinishedAt = time.Now().UnixMilli()
	// A run that could not be closed is still a run the window is waiting to hear the end of: it is
	// told what happened, and the next time the overview opens it reads what was written.
	_ = u.store.SaveRun(ctx, run)
	u.notifier.Publish(TopicRunFinished, run)
}

// attempt sends one request of a run. Every way it can fail is that request's own row: a node that
// cannot be read, a `{{token}}` that resolves to nothing and a server that never answered are one
// line each, and the run goes on. Stopping at the first broken endpoint would hide the twenty
// behind it, which is the opposite of what a run is for.
func (u *UseCase) attempt(
	ctx context.Context,
	item runnable,
	position int64,
	runID string,
) domain.CollectionRunResult {
	node := item.node
	result := domain.CollectionRunResult{NodeID: node.ID, Position: position}

	// The row the tree carries has the method and nothing else; what is sent is the node whole.
	full, err := u.store.Node(ctx, node.ID)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	rec, err := u.sender.Send(ctx, requestFrom(full, item.auth, runID))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if rec.Skipped {
		// A pre-request script said this request must not go out. There is no answer and no error:
		// the row says which of the two happened, and the run counts it as neither.
		result.Skipped = true
		result.Error = "пропущен скриптом"
		return result
	}

	// What the request produced is what the row opens: without it a run can say which endpoint failed
	// and nothing about what it answered.
	result.RecordID = rec.ID
	result.DurationUs = rec.DurationUs
	if rec.Status == 0 {
		// Nothing came back, so the error is all there is to report. A status of zero is not a
		// status: it is the absence of one.
		result.Error = rec.Error
		return result
	}
	status := rec.Status
	result.Status = &status
	result.OK = rec.Error == ""
	result.Error = rec.Error
	return result
}

// requestFrom turns a node into what goes out: the rows a person switched on, the jar beside them,
// and the authorization the levels above it answered with. The URL is the request's own — a query
// string is a URL's rows, not a second copy of them — so the parameters travel for the record and
// do not rewrite the address.
func requestFrom(node domain.CollectionNode, auth *domain.Auth, runID string) RunRequest {
	request := RunRequest{
		Run:     runID,
		NodeID:  node.ID,
		Method:  node.Method,
		URL:     node.URL,
		Body:    node.Body,
		Headers: []domain.HeaderPair{},
		Cookies: orEmptyCookies(node.Cookies),
		Auth:    auth,
	}
	for _, row := range node.Headers {
		if row.Enabled && strings.TrimSpace(row.Name) != "" {
			request.Headers = append(request.Headers, domain.HeaderPair{Name: row.Name, Value: row.Value})
		}
	}
	return request
}

// runnable is one request of a run together with the authorization the walk found above it: the
// tree is what knows about inheritance, and the sender is told the answer rather than the tree.
type runnable struct {
	node domain.CollectionNode
	auth *domain.Auth
}

// requestsUnder lists what a run walks, in the order the tree draws it: depth first, so a folder is
// followed by what is inside it. A request is its own subtree, which is why running a single saved
// request and running a folder are the same call.
//
// A node the collection does not have is not an empty run: it is a node the window knows and this
// tree does not, which is a deletion it has not heard about yet.
func requestsUnder(collection domain.Collection, nodeID string) ([]runnable, error) {
	if nodeID == "" || nodeID == collection.ID {
		return requestsIn(collection.Items, collection.Auth), nil
	}
	node, ok := findNode([]domain.Collection{collection}, nodeID)
	if !ok {
		return nil, fmt.Errorf("узел %s: %w", nodeID, domain.ErrNotFound)
	}
	return requestsIn([]domain.CollectionNode{node}, collection.Auth), nil
}

func requestsIn(nodes []domain.CollectionNode, inherited *domain.Auth) []runnable {
	out := []runnable{}
	for _, node := range nodes {
		at := inherited
		if node.Auth != nil {
			at = node.Auth
		}
		if node.Kind == domain.NodeRequest {
			out = append(out, runnable{node: node, auth: at})
			continue
		}
		out = append(out, requestsIn(node.Items, at)...)
	}
	return out
}

// findCollection looks a collection up by id, which the run needs because the tree it was given
// holds every collection and only one of them is being run.
func findCollection(tree []domain.Collection, id string) (domain.Collection, bool) {
	for _, collection := range tree {
		if collection.ID == id {
			return collection, true
		}
	}
	return domain.Collection{}, false
}
