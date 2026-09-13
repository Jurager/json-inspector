// Package scripting runs the code a collection keeps around its requests: before a request goes out
// and after the answer came back.
//
// What runs is a chain and not a single script: the collection's scripts, then every folder's on the
// way down, then the request's own. A level does not replace what is above it — all of it runs, in
// that order, which is what lets a collection count its requests while one request asserts about its
// own answer.
package scripting

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// KindCollection is what a level calls the collection itself: a node is a folder or a request, and
// the collection is neither.
const KindCollection = "collection"

type UseCase struct {
	engine Engine
	tree   Tree
	store  Store
	vars   Variables
	ids    platform.IDGen

	// The run's own scope, which is what `pm.variables` reads and writes. It lives here because it
	// belongs to no feature that outlives the run: a few strings per run, gone with the process. Only
	// a run whose scripts actually wrote something has an entry, and a script that only reads leaves
	// nothing behind.
	mu   sync.Mutex
	runs map[string]map[string]string
}

func NewUseCase(engine Engine, tree Tree, store Store, vars Variables, ids platform.IDGen) *UseCase {
	return &UseCase{engine: engine, tree: tree, store: store, vars: vars, ids: ids, runs: map[string]map[string]string{}}
}

// Level is one step of a chain: a collection, a folder or a request, and what it runs.
type Level struct {
	NodeID  string         `json:"nodeId"`
	Name    string         `json:"name"`
	Kind    string         `json:"kind"`
	Scripts domain.Scripts `json:"scripts"`
}

// Chain is what runs around a request, outermost first — the collection's scripts, then each folder's
// on the way down, then the request's own. A level with nothing to run is not in it: this is what
// runs, not the places it could have run from.
func (u *UseCase) Chain(ctx context.Context, nodeID string) ([]Level, error) {
	// A request composed on the command line came from nowhere, so nothing is around it.
	if nodeID == "" {
		return []Level{}, nil
	}

	tree, err := u.tree.Collections(ctx)
	if err != nil {
		return nil, err
	}
	for _, collection := range tree {
		// The collection itself: nothing is above it, and what it runs is the whole chain.
		if collection.ID == nodeID {
			scripts, err := u.tree.Scripts(ctx, collection.ID)
			if err != nil {
				return nil, err
			}
			if level, ok := newLevel(collection.ID, collection.Name, KindCollection, scripts); ok {
				return []Level{level}, nil
			}
			return []Level{}, nil
		}

		path, ok := pathTo(collection.Items, nodeID)
		if !ok {
			continue
		}

		chain := []Level{}
		scripts, err := u.tree.Scripts(ctx, collection.ID)
		if err != nil {
			return nil, err
		}
		if level, ok := newLevel(collection.ID, collection.Name, KindCollection, scripts); ok {
			chain = append(chain, level)
		}
		for _, node := range path {
			scripts, err := u.tree.Scripts(ctx, node.ID)
			if err != nil {
				return nil, err
			}
			if level, ok := newLevel(node.ID, node.Name, string(node.Kind), scripts); ok {
				chain = append(chain, level)
			}
		}
		return chain, nil
	}
	return nil, fmt.Errorf("узел %s: %w", nodeID, domain.ErrNotFound)
}

// Before runs the pre-request scripts of everything above a request and answers whether the request
// should be sent at all.
//
// What the scripts do to the request is on the pass, and they all see the same one: a collection's
// script changes what a folder's is handed, and the folder's changes what the request's own sees.
func (u *UseCase) Before(ctx context.Context, pass domain.ScriptPass) (bool, error) {
	return u.half(ctx, pass, domain.ScriptPre)
}

// After runs the post-response scripts of the same chain, in the same order.
//
// It answers with nothing because there is nothing left to say: the answer came back and the request
// is a record already. A report that cannot be written is lost — the alternative is a request that
// failed after it had worked.
func (u *UseCase) After(ctx context.Context, pass domain.ScriptPass) {
	_, _ = u.half(ctx, pass, domain.ScriptPost)
}

// Runs is what the response viewer draws: every script that ran around one record, in the order they
// ran. An empty answer is a request whose collection has no scripts, which is most of them.
func (u *UseCase) Runs(ctx context.Context, recordID string) ([]domain.ScriptRun, error) {
	return u.store.ScriptRuns(ctx, recordID)
}

// half is one side of the chain: the scripts of one scope, level by level. A level whose script is
// empty for this scope is skipped — an empty script is a level with nothing to add, not a level that
// throws the ones above it away — and a level that fails is a report, so the rest still run.
func (u *UseCase) half(ctx context.Context, at domain.ScriptPass, scope domain.ScriptScope) (bool, error) {
	chain, err := u.Chain(ctx, at.NodeID)
	if err != nil {
		return false, err
	}

	skip := false
	for _, level := range chain {
		source := level.Scripts.Pre
		if scope == domain.ScriptPost {
			source = level.Scripts.Post
		}
		if strings.TrimSpace(source) == "" {
			continue
		}
		report, err := u.runScript(ctx, at, level, scope, source)
		if err != nil {
			return false, err
		}
		// Only a pre-request script can call a request off; one that does it after the answer came
		// back is too late to matter, and saying so would be a lie about what happened.
		if scope == domain.ScriptPre {
			skip = skip || report.SkipRequest
		}
	}
	return skip, nil
}

// runScript executes one level's script and writes its report down. Everything that says who ran —
// which record, which node, when — is stamped here: the sandbox knows none of it, and the tab draws
// all of it.
func (u *UseCase) runScript(
	ctx context.Context,
	at domain.ScriptPass,
	level Level,
	scope domain.ScriptScope,
	source string,
) (domain.ScriptRun, error) {
	report := u.engine.Run(domain.ScriptInput{
		Scope:     scope,
		Source:    source,
		Request:   at.Request,
		Response:  at.Response,
		Variables: scriptVariables{ctx: ctx, owner: u, run: at.Run},
	})
	report.ID = u.ids()
	report.RecordID = at.RecordID
	report.NodeID = level.NodeID
	report.Scope = scope
	report.CreatedAt = time.Now().UnixMilli()
	if err := u.store.SaveScriptRun(ctx, report); err != nil {
		return domain.ScriptRun{}, err
	}
	return report, nil
}

// scriptVariables is where a script reads and writes, seen by one script of one run.
type scriptVariables struct {
	ctx   context.Context
	owner *UseCase
	run   string
}

var _ domain.VarStore = scriptVariables{}

// Get answers the run's own scope first and the environment and the globals after it — the order a
// request is resolved in, so a script asking for a name gets what the next request of the run will
// get. Reading either of the other two asks for that one alone, which is what tells a script whether
// a name is set in the environment or only borrowed from the run.
func (v scriptVariables) Get(scope domain.VarScope, name string) (string, bool, error) {
	if scope != domain.ScopeRun {
		return v.owner.vars.Variable(v.ctx, scope, name)
	}
	if value, ok := v.owner.runVariable(v.run, name); ok {
		return value, true, nil
	}
	value, ok, err := v.owner.vars.Variable(v.ctx, domain.ScopeEnvironment, name)
	if err != nil || ok {
		return value, ok, err
	}
	return v.owner.vars.Variable(v.ctx, domain.ScopeGlobals, name)
}

// Set writes where the scope says it goes. A value a script puts in the run's own scope is this run's
// and nobody else's: the next request of the same run sees it, another run does not, and a restart
// forgets it. The other two are stored, which is the point of them — and a secret written here is
// stored like any other value, because the app has nowhere else to keep it.
func (v scriptVariables) Set(scope domain.VarScope, name string, value string) error {
	if scope != domain.ScopeRun {
		return v.owner.vars.SetVariable(v.ctx, scope, name, value)
	}
	v.owner.setRunVariable(v.run, name, value)
	return nil
}

func (u *UseCase) runVariable(run string, name string) (string, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()

	value, ok := u.runs[run][name]
	return value, ok
}

func (u *UseCase) setRunVariable(run string, name string, value string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.runs[run] == nil {
		u.runs[run] = map[string]string{}
	}
	u.runs[run][name] = value
}

// pathTo is the way down from a collection's root to one node. What is above a request is what runs
// before it does, so the order of this walk is the order of the chain.
func pathTo(nodes []domain.CollectionNode, id string) ([]domain.CollectionNode, bool) {
	for _, node := range nodes {
		if node.ID == id {
			return []domain.CollectionNode{node}, true
		}
		if below, ok := pathTo(node.Items, id); ok {
			return append([]domain.CollectionNode{node}, below...), true
		}
	}
	return nil, false
}

func newLevel(id string, name string, kind string, scripts *domain.Scripts) (Level, bool) {
	if scripts.Empty() {
		return Level{}, false
	}
	return Level{NodeID: id, Name: name, Kind: kind, Scripts: *scripts}, true
}
