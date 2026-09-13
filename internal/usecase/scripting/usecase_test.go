package scripting

import (
	"context"
	"errors"
	"strings"
	"testing"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// fakeEngine is the sandbox without goja: it keeps what it was asked to run and answers with a report
// the test describes, so a test says what a script did without writing one.
type fakeEngine struct {
	ran   []domain.ScriptInput
	onRun func(in domain.ScriptInput) domain.ScriptRun
}

func (f *fakeEngine) Run(in domain.ScriptInput) domain.ScriptRun {
	f.ran = append(f.ran, in)
	if f.onRun != nil {
		return f.onRun(in)
	}
	return domain.ScriptRun{Scope: in.Scope, OK: true, Logs: []domain.ScriptLog{}, Tests: []domain.TestResult{}}
}

// sources is what ran, in the order it ran, which is what a chain is about.
func (f *fakeEngine) sources() []string {
	out := []string{}
	for _, in := range f.ran {
		out = append(out, in.Source)
	}
	return out
}

// fakeTree is the tree without a database: one list of collections and what each level runs. A level
// nobody put anything in answers nothing, the way the NULL column does.
type fakeTree struct {
	collections []domain.Collection
	scripts     map[string]*domain.Scripts
}

func (f *fakeTree) Collections(context.Context) ([]domain.Collection, error) {
	return f.collections, nil
}

func (f *fakeTree) Scripts(_ context.Context, id string) (*domain.Scripts, error) {
	return f.scripts[id], nil
}

// fakeStore keeps the reports, which is what the response viewer asks by.
type fakeStore struct {
	runs []domain.ScriptRun
	// fail is the database being broken, which is the only way writing a report can fail.
	fail bool
}

func (f *fakeStore) SaveScriptRun(_ context.Context, run domain.ScriptRun) error {
	if f.fail {
		return errors.New("база недоступна")
	}
	f.runs = append(f.runs, run)
	return nil
}

func (f *fakeStore) ScriptRuns(_ context.Context, recordID string) ([]domain.ScriptRun, error) {
	out := []domain.ScriptRun{}
	for _, run := range f.runs {
		if run.RecordID == recordID {
			run.RecordID = recordID
			out = append(out, run)
		}
	}
	return out, nil
}

// fakeVariables is the environments screen without a database. What a script writes is kept here as
// well as in the values, because where it was written is half of what these tests are about.
type fakeVariables struct {
	values  map[domain.VarScope]map[string]string
	refused map[domain.VarScope]string
	writes  []write
}

type write struct {
	scope domain.VarScope
	name  string
	value string
}

func newFakeVariables() *fakeVariables {
	return &fakeVariables{values: map[domain.VarScope]map[string]string{}, refused: map[domain.VarScope]string{}}
}

func (f *fakeVariables) with(scope domain.VarScope, name string, value string) *fakeVariables {
	if f.values[scope] == nil {
		f.values[scope] = map[string]string{}
	}
	f.values[scope][name] = value
	return f
}

func (f *fakeVariables) Variable(_ context.Context, scope domain.VarScope, name string) (string, bool, error) {
	value, ok := f.values[scope][name]
	return value, ok, nil
}

func (f *fakeVariables) SetVariable(_ context.Context, scope domain.VarScope, name string, value string) error {
	if reason, refused := f.refused[scope]; refused {
		return errors.New(reason)
	}
	f.with(scope, name, value)
	f.writes = append(f.writes, write{scope: scope, name: name, value: value})
	return nil
}

// seedTree is one collection with a folder inside it and a request inside the folder, plus a request
// at the root: every shape a chain has to keep straight.
func seedTree() *fakeTree {
	return &fakeTree{
		collections: []domain.Collection{
			{
				ID: "col-1", Name: "Пользователи",
				Items: []domain.CollectionNode{
					{ID: "f-1", CollectionID: "col-1", Kind: domain.NodeFolder, Name: "Админ", Items: []domain.CollectionNode{
						{ID: "r-1", CollectionID: "col-1", ParentID: "f-1", Kind: domain.NodeRequest, Name: "Список"},
					}},
					{ID: "r-2", CollectionID: "col-1", Kind: domain.NodeRequest, Name: "Один"},
				},
			},
			{ID: "col-2", Name: "Заказы", Items: []domain.CollectionNode{}},
		},
		scripts: map[string]*domain.Scripts{
			"col-1": {Pre: "console.log('коллекция');", Post: "console.log('после коллекции');"},
			"f-1":   {Pre: "console.log('папка');"},
			"r-1":   {Pre: "console.log('запрос');"},
		},
	}
}

func newTest() (*UseCase, *fakeEngine, *fakeTree, *fakeStore, *fakeVariables) {
	engine := &fakeEngine{}
	tree := seedTree()
	store := &fakeStore{}
	vars := newFakeVariables()
	uc := NewUseCase(engine, tree, store, vars, platform.NewIDGen())
	return uc, engine, tree, store, vars
}

func pass(nodeID string) domain.ScriptPass {
	return domain.ScriptPass{
		Run:      "run-1",
		RecordID: "rec-1",
		NodeID:   nodeID,
		Request:  &domain.ScriptRequest{Method: "GET", URL: "https://api.example.com/users"},
	}
}

// What runs around a request is everything above it, outermost first — the collection's scripts, then
// the folder's, then the request's own. A level that runs nothing is not in the chain at all.
func TestTheChainIsEverythingAboveTheRequest(t *testing.T) {
	uc, _, _, _, _ := newTest()

	chain, err := uc.Chain(context.Background(), "r-1")
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	if len(chain) != 3 {
		t.Fatalf("chain = %+v, want the collection, the folder and the request", chain)
	}
	if chain[0].NodeID != "col-1" || chain[0].Kind != KindCollection || chain[0].Name != "Пользователи" {
		t.Errorf("chain[0] = %+v, want the collection first", chain[0])
	}
	if chain[1].NodeID != "f-1" || chain[1].Kind != string(domain.NodeFolder) {
		t.Errorf("chain[1] = %+v, want the folder next", chain[1])
	}
	if chain[2].NodeID != "r-1" || chain[2].Kind != string(domain.NodeRequest) {
		t.Errorf("chain[2] = %+v, want the request last", chain[2])
	}
}

func TestAChainIsEmptyWhenNothingRuns(t *testing.T) {
	uc, _, _, _, _ := newTest()
	ctx := context.Background()

	// The request at the root: the collection above it has scripts, the request itself has none.
	chain, err := uc.Chain(ctx, "r-2")
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	if len(chain) != 1 || chain[0].NodeID != "col-1" {
		t.Errorf("chain = %+v, want only the collection", chain)
	}

	// A collection is the top of its own tree, so what it runs is the whole chain.
	chain, err = uc.Chain(ctx, "col-1")
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	if len(chain) != 1 || chain[0].Kind != KindCollection {
		t.Errorf("chain of a collection = %+v, want itself and nothing above it", chain)
	}
	if chain, err := uc.Chain(ctx, "col-2"); err != nil || len(chain) != 0 {
		t.Errorf("chain of a collection with no scripts = %+v, %v, want nothing", chain, err)
	}
	// A request the command line composed came from nowhere, so nothing is above it.
	if chain, err := uc.Chain(ctx, ""); err != nil || len(chain) != 0 {
		t.Errorf("chain of a request from nowhere = %+v, %v, want nothing", chain, err)
	}
	if _, err := uc.Chain(ctx, "нет-такого"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("chain of an unknown node = %v, want ErrNotFound", err)
	}
}

func TestBeforeRunsEveryPreScriptInOrder(t *testing.T) {
	uc, engine, _, store, _ := newTest()

	asked := pass("r-1")
	skip, err := uc.Before(context.Background(), &asked)
	if err != nil {
		t.Fatalf("Before: %v", err)
	}
	if skip {
		t.Error("the run was called off by a script that said nothing of the kind")
	}

	want := []string{"console.log('коллекция');", "console.log('папка');", "console.log('запрос');"}
	if got := engine.sources(); !equal(got, want) {
		t.Errorf("scripts that ran = %q, want %q", got, want)
	}
	for _, in := range engine.ran {
		if in.Scope != domain.ScriptPre {
			t.Errorf("scope = %q, want every one of them before the request", in.Scope)
		}
	}

	// Nothing is written yet: a report hangs off the record it ran around, and the record does not
	// exist until the request has been answered.
	if len(store.runs) != 0 {
		t.Errorf("%d report(s) were written before the record they belong to", len(store.runs))
	}
	if len(asked.Ran) != 3 {
		t.Errorf("passes that ran = %d, want the three carried to the other half", len(asked.Ran))
	}
}

// A script that changes the request changes what the ones after it see: the collection's script runs
// first and the request's own sees what the folder left.
func TestWhatAScriptChangesTheNextOneSees(t *testing.T) {
	uc, engine, _, _, _ := newTest()
	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		in.Request.Headers = append(in.Request.Headers, domain.HeaderPair{Name: "X-Из", Value: in.Source})
		return domain.ScriptRun{Scope: in.Scope, OK: true}
	}

	asked := pass("r-1")
	if _, err := uc.Before(context.Background(), &asked); err != nil {
		t.Fatalf("Before: %v", err)
	}

	if len(asked.Request.Headers) != 3 {
		t.Fatalf("headers = %+v, want one per level", asked.Request.Headers)
	}
	if asked.Request.Headers[2].Value != "console.log('запрос');" {
		t.Errorf("headers = %+v, want what the request's own script added last", asked.Request.Headers)
	}
}

func TestAPreRequestScriptCanCallTheRequestOff(t *testing.T) {
	uc, engine, _, _, _ := newTest()
	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		return domain.ScriptRun{Scope: in.Scope, OK: true, SkipRequest: in.Source == "console.log('папка');"}
	}

	asked := pass("r-1")
	skip, err := uc.Before(context.Background(), &asked)
	if err != nil {
		t.Fatalf("Before: %v", err)
	}
	if !skip {
		t.Error("the folder's script called the request off and the run did not hear it")
	}
	// Saying so does not stop the chain: the scripts below it still run, and one of them may undo it.
	if len(engine.ran) != 3 {
		t.Errorf("%d scripts ran, want all three", len(engine.ran))
	}
}

// Only a pre-request script can call a request off: one that says so after the answer came back is
// too late, and saying otherwise would be a lie about what happened.
func TestAPostResponseScriptCannotCallTheRequestOff(t *testing.T) {
	uc, engine, _, _, _ := newTest()
	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		return domain.ScriptRun{Scope: in.Scope, OK: true, SkipRequest: true}
	}

	after := pass("r-1")
	after.Response = &domain.Response{Status: 200}
	uc.After(context.Background(), after)

	// After answers with nothing at all, so the flag has nowhere to go — what it must not do is
	// reach the caller, and the caller is the thing that is not here.
	if len(engine.ran) == 0 {
		t.Fatal("the post-response scripts did not run")
	}
	for _, in := range engine.ran {
		if in.Response == nil {
			t.Error("a post-response script was handed no answer")
		}
	}
}

func TestAfterRunsThePostScriptsAndKeepsTheReports(t *testing.T) {
	uc, engine, _, store, _ := newTest()

	after := pass("r-1")
	after.Response = &domain.Response{Status: 200, Body: `{"data":[]}`}
	uc.After(context.Background(), after)

	// The collection and the folder have post scripts; the request's own has none.
	if len(engine.ran) != 1 || engine.ran[0].Source != "console.log('после коллекции');" {
		t.Fatalf("scripts that ran = %q, want the collection's", engine.sources())
	}

	runs, err := uc.Runs(context.Background(), "rec-1")
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("reports = %+v, want one", runs)
	}
	run := runs[0]
	if run.ID == "" || run.RecordID != "rec-1" || run.NodeID != "col-1" || run.Scope != domain.ScriptPost {
		t.Errorf("report = %+v, want it named by what, where and when", run)
	}
	if run.CreatedAt == 0 {
		t.Errorf("report = %+v, want it stamped", run)
	}
	if len(store.runs) != 1 {
		t.Errorf("the store kept %d reports, want 1", len(store.runs))
	}
}

// A store that refuses to keep a report is not a reason to fail a request: the answer came back and
// the request is a record, and a report lost after that is the smaller loss. Neither half may panic
// over it, and the first half may not even notice — it writes nothing.
func TestAStoreThatCannotKeepReportsDoesNotFailTheRequest(t *testing.T) {
	uc, _, _, store, _ := newTest()
	store.fail = true

	asked := pass("r-1")
	if _, err := uc.Before(context.Background(), &asked); err != nil {
		t.Fatalf("Before: %v", err)
	}
	asked.Response = &domain.Response{Status: 200}
	uc.After(context.Background(), asked)
}

// The run's own scope is the run's: the next request of the same run sees what the last one wrote,
// and a run that starts after it does not.
func TestTheRunScopeBelongsToTheRun(t *testing.T) {
	uc, engine, _, _, _ := newTest()
	ctx := context.Background()

	seen := ""
	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		if in.Source == "console.log('коллекция');" {
			_ = in.Variables.Set(domain.ScopeRun, "page", "2")
		}
		if in.Source == "console.log('запрос');" {
			if value, ok, _ := in.Variables.Get(domain.ScopeRun, "page"); ok {
				seen = value
			}
		}
		return domain.ScriptRun{Scope: in.Scope, OK: true}
	}
	first := pass("r-1")
	if _, err := uc.Before(ctx, &first); err != nil {
		t.Fatalf("Before: %v", err)
	}
	if seen != "2" {
		t.Errorf("the request's own script saw %q, want what the collection wrote earlier in the run", seen)
	}

	// The same request, another run: nothing the first one wrote is there.
	other := pass("r-1")
	other.Run = "run-2"
	found := false
	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		if _, ok, _ := in.Variables.Get(domain.ScopeRun, "page"); ok {
			found = true
		}
		return domain.ScriptRun{Scope: in.Scope, OK: true}
	}
	if _, err := uc.Before(ctx, &other); err != nil {
		t.Fatalf("Before: %v", err)
	}
	if found {
		t.Error("the second run saw what the first one wrote, and a run's scope is its own")
	}
}

// Reading `pm.variables` looks through the environment and the globals after the run's own scope,
// which is the order the next request of the run will resolve in.
func TestReadingAVariableLooksThroughTheScopes(t *testing.T) {
	uc, engine, _, _, vars := newTest()
	vars.with(domain.ScopeEnvironment, "base", "https://api.example.com")
	vars.with(domain.ScopeGlobals, "token", "from-globals")
	vars.with(domain.ScopeGlobals, "base", "https://globals.example.com")

	asked := map[string]string{}
	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		for _, name := range []string{"base", "token", "нет"} {
			value, ok, err := in.Variables.Get(domain.ScopeRun, name)
			if err != nil {
				t.Errorf("Get(%s): %v", name, err)
			}
			if ok {
				asked[name] = value
			}
		}
		return domain.ScriptRun{Scope: in.Scope, OK: true}
	}

	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		for _, name := range []string{"base", "token", "нет"} {
			value, ok, err := in.Variables.Get(domain.ScopeRun, name)
			if err != nil {
				t.Errorf("Get(%s): %v", name, err)
			}
			if ok {
				asked[name] = value
			}
		}
		return domain.ScriptRun{Scope: in.Scope, OK: true}
	}

	first := pass("r-1")
	if _, err := uc.Before(context.Background(), &first); err != nil {
		t.Fatalf("Before: %v", err)
	}
	if asked["base"] != "https://api.example.com" {
		t.Errorf("base = %q, want the environment to win over the globals", asked["base"])
	}
	if asked["token"] != "from-globals" {
		t.Errorf("token = %q, want it found in the globals", asked["token"])
	}
	if _, found := asked["нет"]; found {
		t.Error("a name nobody set was answered with something")
	}
}

// The run's own scope is kept here; the environment and the globals are somebody else's, and writing
// to them goes through the port.
func TestWritingAVariableGoesWhereTheScopeSays(t *testing.T) {
	uc, engine, _, _, vars := newTest()
	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		if in.Source != "console.log('коллекция');" {
			return domain.ScriptRun{Scope: in.Scope, OK: true}
		}
		if err := in.Variables.Set(domain.ScopeEnvironment, "token", "в-окружении"); err != nil {
			t.Errorf("Set(environment): %v", err)
		}
		if err := in.Variables.Set(domain.ScopeGlobals, "auth", "в-глобалах"); err != nil {
			t.Errorf("Set(globals): %v", err)
		}
		return domain.ScriptRun{Scope: in.Scope, OK: true}
	}

	asked := pass("r-1")
	if _, err := uc.Before(context.Background(), &asked); err != nil {
		t.Fatalf("Before: %v", err)
	}
	if len(vars.writes) != 2 {
		t.Fatalf("writes = %+v, want one per scope", vars.writes)
	}
	if vars.writes[0].scope != domain.ScopeEnvironment || vars.writes[0].name != "token" {
		t.Errorf("write = %+v, want the environment one first", vars.writes[0])
	}
	if vars.writes[1].scope != domain.ScopeGlobals || vars.writes[1].value != "в-глобалах" {
		t.Errorf("write = %+v, want the globals one", vars.writes[1])
	}
	// The run's own scope is this feature's memory and goes nowhere near the port.
	if len(vars.values[domain.ScopeRun]) != 0 {
		t.Errorf("the run scope = %+v, want it kept here and not written through the port", vars.values[domain.ScopeRun])
	}
}

// A scope that cannot be written is the script's business to see, so the error travels out of the
// port and into the sandbox rather than being swallowed here.
func TestAScopeThatRefusesAWriteSaysSo(t *testing.T) {
	uc, engine, _, _, vars := newTest()
	vars.refused[domain.ScopeEnvironment] = "окружение только для чтения"

	var refused error
	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		refused = in.Variables.Set(domain.ScopeEnvironment, "token", "1")
		return domain.ScriptRun{Scope: in.Scope, OK: true}
	}

	asked := pass("r-1")
	if _, err := uc.Before(context.Background(), &asked); err != nil {
		t.Fatalf("Before: %v", err)
	}
	if refused == nil || !strings.Contains(refused.Error(), "только для чтения") {
		t.Errorf("error = %v, want the reason the scope gave", refused)
	}
}

// A script that fails is a report, not the end of the chain: the levels below it still run, and a
// collection that counts its requests keeps counting even when a folder's script is broken.
func TestAFailedScriptDoesNotStopTheChain(t *testing.T) {
	uc, engine, _, store, _ := newTest()
	engine.onRun = func(in domain.ScriptInput) domain.ScriptRun {
		if in.Source == "console.log('папка');" {
			return domain.ScriptRun{Scope: in.Scope, OK: false, Error: "ReferenceError: nope"}
		}
		return domain.ScriptRun{Scope: in.Scope, OK: true}
	}

	asked := pass("r-1")
	if _, err := uc.Before(context.Background(), &asked); err != nil {
		t.Fatalf("Before: %v", err)
	}
	asked.Response = &domain.Response{Status: 200}
	uc.After(context.Background(), asked)

	// The three pre-request scripts of the chain and the collection's post-response one: the folder's
	// failure stopped neither the levels below it nor the second half.
	if len(engine.ran) != 4 {
		t.Fatalf("%d scripts ran, want all four", len(engine.ran))
	}
	if len(store.runs) != 4 {
		t.Fatalf("%d reports were kept, want one per script", len(store.runs))
	}
	if store.runs[1].OK || store.runs[1].Error != "ReferenceError: nope" {
		t.Errorf("report = %+v, want the failure the folder's script came with", store.runs[1])
	}
}

func equal(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
