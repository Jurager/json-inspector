package environment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"json-inspector/internal/domain"
	"json-inspector/internal/dotenv"
	"json-inspector/internal/platform"
)

// fakeStore is the in-memory double for the feature's port. It keeps the same rules the SQL does —
// variables belong to a scope, the active environment is a setting, an import is claimed once — so
// the tests below are about the use case, not about SQLite.
type fakeStore struct {
	envs    []domain.Environment
	vars    map[string]fakeVar
	active  string
	imports map[string]domain.ImportStatus
}

type fakeVar struct {
	scope domain.EnvScope
	v     domain.Variable
}

func newFakeStore() *fakeStore {
	return &fakeStore{vars: map[string]fakeVar{}, imports: map[string]domain.ImportStatus{}}
}

// fakeScope answers with the workspace the test is working in. The store below keeps one flat
// state and ignores the id: what is being tested here is the use case, and the split between
// workspaces is the SQL's own test.
type fakeScope struct{ id string }

func (f fakeScope) ActiveWorkspace(context.Context) (string, error) {
	if f.id == "" {
		return domain.WorkspacePersonalID, nil
	}
	return f.id, nil
}

// fakeHome answers with the workspace the installation began in, which every test here is: the
// import it serves is about this installation's own data, not about the space on screen.
type fakeHome struct{ id string }

func (f fakeHome) FirstWorkspace(context.Context) (string, error) {
	if f.id == "" {
		return domain.WorkspacePersonalID, nil
	}
	return f.id, nil
}

func (f *fakeStore) EnvState(context.Context, string) (domain.EnvState, error) {
	state := domain.EnvState{Environments: []domain.Environment{}, Globals: []domain.Variable{},
		ActiveID: f.active}
	for _, env := range f.envs {
		copied := env
		copied.Vars = []domain.Variable{}
		for _, entry := range sortedVars(f.vars) {
			if entry.scope.Environment == env.ID {
				copied.Vars = append(copied.Vars, entry.v)
			}
		}
		state.Environments = append(state.Environments, copied)
	}
	for _, entry := range sortedVars(f.vars) {
		if entry.scope.Environment == "" {
			state.Globals = append(state.Globals, entry.v)
		}
	}
	return state, nil
}

// sortedVars keeps the map deterministic so positions and order are comparable.
func sortedVars(vars map[string]fakeVar) []fakeVar {
	out := make([]fakeVar, 0, len(vars))
	for _, v := range vars {
		out = append(out, v)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].v.Position > out[j].v.Position; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func (f *fakeStore) SaveEnvironment(_ context.Context, _ string, env domain.Environment) error {
	for i := range f.envs {
		if f.envs[i].ID == env.ID {
			f.envs[i] = env
			return nil
		}
	}
	f.envs = append(f.envs, env)
	return nil
}

func (f *fakeStore) DeleteEnvironment(_ context.Context, _ string, id string) error {
	kept := f.envs[:0]
	for _, env := range f.envs {
		if env.ID != id {
			kept = append(kept, env)
		}
	}
	f.envs = kept
	for key, entry := range f.vars {
		if entry.scope.Environment == id {
			delete(f.vars, key)
		}
	}
	return nil
}

func (f *fakeStore) SaveVariable(
	_ context.Context,
	_ string,
	scope domain.EnvScope,
	v domain.Variable,
) error {
	f.vars[v.ID] = fakeVar{scope: scope, v: v}
	return nil
}

func (f *fakeStore) DeleteVariable(_ context.Context, _ string, id string) error {
	delete(f.vars, id)
	return nil
}

func (f *fakeStore) VariableValue(_ context.Context, _ string, id string) (string, error) {
	entry, ok := f.vars[id]
	if !ok {
		return "", domain.ErrNotFound
	}
	return entry.v.Value, nil
}

func (f *fakeStore) SetActiveEnvironment(_ context.Context, _ string, id string) error {
	f.active = id
	return nil
}

func (f *fakeStore) ClaimImport(_ context.Context, source string) (bool, error) {
	if status, ok := f.imports[source]; ok && status != domain.ImportPending {
		return false, nil
	}
	f.imports[source] = domain.ImportPending
	return true, nil
}

func (f *fakeStore) FinishImport(
	_ context.Context,
	source string,
	status domain.ImportStatus,
	_ string,
) error {
	f.imports[source] = status
	return nil
}

func newUseCase(t *testing.T) (*UseCase, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	ids := platform.NewIDGen()
	return NewUseCase(store, fakeScope{}, fakeHome{}, ids), store
}

func seed(t *testing.T, u *UseCase, name string) (domain.EnvState, domain.Environment) {
	t.Helper()
	state, err := u.Create(context.Background(), EnvironmentDraft{Name: name})
	if err != nil {
		t.Fatalf("Create(%q): %v", name, err)
	}
	return state, state.Environments[len(state.Environments)-1]
}

func TestCreateValidatesAndActivates(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()

	for _, bad := range []string{"", "   ", strings.Repeat("x", maxNameLength+1)} {
		if _, err := u.Create(ctx, EnvironmentDraft{Name: bad}); !errors.Is(err, domain.ErrNotAllowed) {
			t.Errorf("Create(%q) = %v, want domain.ErrNotAllowed", bad, err)
		}
	}

	state, env := seed(t, u, "Local")
	if env.Name != "Local" {
		t.Errorf("created environment = %+v, want the name it was given", env)
	}
	if state.ActiveID != env.ID {
		t.Errorf("activeId = %q, want the new environment", state.ActiveID)
	}

	// Each one lands after the last, which is the order the list draws them in.
	state, second := seed(t, u, "Prod")
	if second.Position <= env.Position {
		t.Errorf("positions = %d then %d, want the newer one after", env.Position, second.Position)
	}
	if state.ActiveID != second.ID {
		t.Errorf("activeId = %q, want the newest environment", state.ActiveID)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()
	_, env := seed(t, u, "Local")

	name := "Prod · EU"
	readonly := true
	state, err := u.Update(ctx, env.ID,
		EnvironmentPatch{Name: &name, Readonly: &readonly, Color: ptr("red")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := state.Environments[0]
	if got.Name != name || !got.Readonly || got.Color != "red" {
		t.Errorf("after Update: %+v", got)
	}

	if _, err := u.Update(ctx, "nope", EnvironmentPatch{Name: &name}); !errors.Is(err,
		domain.ErrNotFound) {
		t.Errorf("Update of a missing environment = %v, want ErrNotFound", err)
	}

	if _, err := u.Delete(ctx, env.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	state, err = u.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(state.Environments) != 0 || state.ActiveID != "" {
		t.Errorf("after Delete: %+v, want no environments and no active one", state)
	}
}

func TestVariablesAndVarValueKeepsSecrets(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()
	_, env := seed(t, u, "Local")

	state, err := u.AddVariable(ctx, domain.EnvScope{Environment: env.ID},
		VariableDraft{Kind: domain.VariableSecret})
	if err != nil {
		t.Fatalf("AddVariable: %v", err)
	}
	v := state.Environments[0].Vars[0]

	state, err = u.UpdateVariable(ctx, domain.EnvScope{Environment: env.ID}, VariablePatch{
		ID: v.ID, Name: "token", Kind: domain.VariableSecret, Enabled: true, Value: "s3cret",
		SetValue: true,
	})
	if err != nil {
		t.Fatalf("UpdateVariable: %v", err)
	}
	v = state.Environments[0].Vars[0]
	if v.Name != "token" || v.Value != "" || !v.HasValue {
		t.Errorf("a secret came back as %+v, want a name, no value and hasValue", v)
	}

	// Renaming a secret without sending its value back keeps the stored one.
	state, err = u.UpdateVariable(ctx, domain.EnvScope{Environment: env.ID}, VariablePatch{
		ID: v.ID, Name: "api_token", Kind: domain.VariableSecret, Enabled: true,
	})
	if err != nil {
		t.Fatalf("UpdateVariable (rename): %v", err)
	}
	if value, err := u.Reveal(ctx, v.ID); err != nil || value != "s3cret" {
		t.Errorf("after a rename: Reveal = %q, %v; want the value kept", value, err)
	}
	if state.Environments[0].Vars[0].Name != "api_token" {
		t.Errorf("the rename did not take: %+v", state.Environments[0].Vars[0])
	}

	// A name the grammar would never resolve is refused here, not silently stored.
	if _, err := u.UpdateVariable(ctx, domain.EnvScope{Environment: env.ID}, VariablePatch{
		ID: v.ID, Name: "not a name", Kind: domain.VariableText, Enabled: true, SetValue: true,
	}); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("a bad name = %v, want ErrNotAllowed", err)
	}

	if _, err := u.RemoveVariable(ctx, domain.EnvScope{Environment: env.ID}, v.ID); err != nil {
		t.Fatalf("RemoveVariable: %v", err)
	}
	state, _ = u.Snapshot(ctx)
	if len(state.Environments[0].Vars) != 0 {
		t.Errorf("the variable is still there: %+v", state.Environments[0].Vars)
	}
}

func TestResolutionOrderAndMasking(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()
	_, env := seed(t, u, "Local")

	if _, err := u.AddVariable(ctx, domain.EnvScope{},
		VariableDraft{Kind: domain.VariableText}); err != nil {
		t.Fatalf("AddVariable (globals): %v", err)
	}
	if _, err := u.AddVariable(ctx, domain.EnvScope{Environment: env.ID},
		VariableDraft{Kind: domain.VariableText}); err != nil {
		t.Fatalf("AddVariable: %v", err)
	}
	if _, err := u.AddVariable(ctx, domain.EnvScope{Environment: env.ID},
		VariableDraft{Kind: domain.VariableSecret}); err != nil {
		t.Fatalf("AddVariable: %v", err)
	}

	state, _ := u.Snapshot(ctx)
	global := state.Globals[0]
	over := state.Environments[0].Vars[0]
	secret := state.Environments[0].Vars[1]

	fill := func(scope domain.EnvScope, v domain.Variable, name, value string,
		kind domain.VariableKind) {
		t.Helper()
		if _, err := u.UpdateVariable(ctx, scope, VariablePatch{
			ID: v.ID, Name: name, Kind: kind, Enabled: true, Value: value, SetValue: true,
		}); err != nil {
			t.Fatalf("UpdateVariable(%s): %v", name, err)
		}
	}
	fill(domain.EnvScope{}, global, "token", "global-value", domain.VariableText)
	fill(domain.EnvScope{Environment: env.ID}, over, "token", "env-value", domain.VariableText)
	fill(domain.EnvScope{Environment: env.ID}, secret, "secret", "s3cret", domain.VariableSecret)

	// The environment wins over the globals: the order the design names is
	// Request → Environment → Globals.
	resolved, err := u.SubstituteTexts(ctx, nil, []string{"{{token}}"}, false)
	if err != nil {
		t.Fatalf("SubstituteTexts: %v", err)
	}
	if resolved[0] != "env-value" {
		t.Errorf("token resolved to %q, want the environment's value", resolved[0])
	}

	// Substitution: the request gets the values, everything that outlives it gets the mask — and a
	// secret is the difference between the two.
	sent, err := u.SubstituteTexts(ctx, nil, []string{"{{token}}/{{secret}}"}, false)
	if err != nil {
		t.Fatalf("SubstituteTexts: %v", err)
	}
	if sent[0] != "env-value/s3cret" {
		t.Errorf("for sending = %q, want the real values", sent[0])
	}
	masked, err := u.SubstituteTexts(ctx, nil, []string{"{{token}}/{{secret}}"}, true)
	if err != nil {
		t.Fatalf("SubstituteTexts (masked): %v", err)
	}
	if masked[0] != "env-value/"+secretMask {
		t.Errorf("masked = %q, want the secret replaced by the mask", masked[0])
	}

	// Several texts at once, and a name that repeats across them is one thing missing.
	missing, err := u.Missing(ctx, nil, []string{"{{token}}/{{nope}}", "{{nope}}/{{other}}"})
	if err != nil {
		t.Fatalf("Missing: %v", err)
	}
	if len(missing) != 2 || missing[0] != "nope" || missing[1] != "other" {
		t.Errorf("Missing = %v, want nope once and then other", missing)
	}

	// A disabled variable stops resolving, which is what the enabled flag is for.
	if _, err := u.UpdateVariable(ctx, domain.EnvScope{Environment: env.ID}, VariablePatch{
		ID: over.ID, Name: "token", Kind: domain.VariableText, Enabled: false, Value: "env-value",
		SetValue: true,
	}); err != nil {
		t.Fatalf("UpdateVariable (disable): %v", err)
	}
	again, err := u.SubstituteTexts(ctx, nil, []string{"{{token}}"}, false)
	if err != nil {
		t.Fatalf("SubstituteTexts: %v", err)
	}
	if again[0] != "global-value" {
		t.Errorf("with the override disabled, token = %q, want the global", again[0])
	}
}

// A collection answers for the requests inside it, and its answer stands over the environment's:
// a level that names `baseUrl` means that value for everything below, which is what lets a
// collection be carried from one environment to another and still mean what it says. A name it does
// not answer is still the environment's, and a row switched off answers nothing — the same rule the
// environment's own rows follow.
func TestCollectionVariablesStandOverTheEnvironment(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()
	_, env := seed(t, u, "Local")

	if _, err := u.AddVariable(ctx, domain.EnvScope{Environment: env.ID},
		VariableDraft{Kind: domain.VariableText}); err != nil {
		t.Fatalf("AddVariable: %v", err)
	}
	if _, err := u.AddVariable(ctx, domain.EnvScope{Environment: env.ID},
		VariableDraft{Kind: domain.VariableText}); err != nil {
		t.Fatalf("AddVariable: %v", err)
	}

	snapshot, err := u.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	envURL, envPage := snapshot.Environments[0].Vars[0], snapshot.Environments[0].Vars[1]
	set := func(v domain.Variable, name, value string) {
		t.Helper()
		if _, err := u.UpdateVariable(ctx, domain.EnvScope{Environment: env.ID}, VariablePatch{
			ID: v.ID, Name: name, Kind: domain.VariableText, Enabled: true, Value: value,
			SetValue: true,
		}); err != nil {
			t.Fatalf("UpdateVariable(%s): %v", name, err)
		}
	}
	set(envURL, "baseUrl", "https://env")
	set(envPage, "page", "3")

	above := []domain.Variable{
		{Name: "baseUrl", Value: "https://collection", Kind: domain.VariableText, Enabled: true},
	}

	resolved, err := u.SubstituteTexts(ctx, above, []string{"{{baseUrl}}/{{page}}"}, false)
	if err != nil {
		t.Fatalf("SubstituteTexts: %v", err)
	}
	if resolved[0] != "https://collection/3" {
		t.Errorf("resolved = %q, want the collection's base and the environment's page",
			resolved[0])
	}

	// The collection's own row switched off is a level that answered nothing, so the environment is
	// what stands again.
	above[0].Enabled = false
	off, err := u.SubstituteTexts(ctx, above, []string{"{{baseUrl}}"}, false)
	if err != nil {
		t.Fatalf("SubstituteTexts(disabled): %v", err)
	}
	if off[0] != "https://env" {
		t.Errorf("resolved = %q, want the environment's base", off[0])
	}

	// And what a request in no collection asks is unchanged: no levels above it, no answers.
	none, err := u.SubstituteTexts(ctx, nil, []string{"{{baseUrl}}"}, false)
	if err != nil {
		t.Fatalf("SubstituteTexts(none): %v", err)
	}
	if none[0] != "https://env" {
		t.Errorf("resolved = %q, want the environment's base", none[0])
	}
}

func TestImportEntriesMergesByName(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()
	_, env := seed(t, u, "Local")
	scope := domain.EnvScope{Environment: env.ID}

	if _, err := u.ImportEntries(ctx, scope, []dotenv.Entry{
		{Name: "base_url", Value: "https://one.example.com"},
		{Name: "api_token", Value: "t1", Secret: true},
	}); err != nil {
		t.Fatalf("ImportEntries: %v", err)
	}
	state, err := u.ImportEntries(ctx, scope, []dotenv.Entry{
		{Name: "base_url", Value: "https://two.example.com"},
		{Name: "page_size", Value: "10"},
	})
	if err != nil {
		t.Fatalf("ImportEntries (second): %v", err)
	}

	vars := state.Environments[0].Vars
	if len(vars) != 3 {
		t.Fatalf("vars = %d, want three after a merge", len(vars))
	}
	byName := map[string]domain.Variable{}
	for _, v := range vars {
		byName[v.Name] = v
	}
	if byName["base_url"].Value != "https://two.example.com" {
		t.Errorf("base_url = %q, want the imported value to replace the old one",
			byName["base_url"].Value)
	}
	if byName["api_token"].Kind != domain.VariableSecret {
		t.Errorf("the secret lost its kind: %+v", byName["api_token"])
	}
	if byName["page_size"].Value != "10" {
		t.Errorf("the new variable was not added: %+v", byName["page_size"])
	}
}

func TestImportLegacyRunsOnceAndLeavesSecretsEmpty(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()

	payload := `{
	  "environments": [{"id": "env-1", "name": "Local", "readonly": false, "vars": [
	    {"id": "v1", "name": "base_url", "value": "https://api.example.com", "kind": "text",
	    	"enabled": true},
	    {"id": "v2", "name": "token", "value": "stale", "kind": "secret", "enabled": true}
	  ]}],
	  "globals": [{"id": "g1", "name": "page_size", "value": "10", "kind": "text", "enabled": true}],
	  "activeId": "env-1"
	}`

	report, err := u.ImportLegacy(ctx, payload)
	if err != nil {
		t.Fatalf("ImportLegacy: %v", err)
	}
	if !report.Completed || report.Environments != 1 || report.Variables != 3 || report.Secrets != 1 {
		t.Errorf("report = %+v, want one environment, three variables and one secret", report)
	}
	// A secret is named rather than carried: what it held lived in the keychain, which is not read
	// any more, and the payload's own `value` for it is a stale copy nothing kept up to date.
	if len(report.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one about the secret", report.Warnings)
	}

	state, _ := u.Snapshot(ctx)
	if state.ActiveID != "env-1" {
		t.Errorf("activeId = %q, want env-1", state.ActiveID)
	}
	if value, err := u.Reveal(ctx, "v2"); err != nil || value != "" {
		t.Errorf("the secret = %q, %v; want no value at all", value, err)
	}

	// A second run is a no-op: the claim is what makes the import happen once.
	again, err := u.ImportLegacy(ctx, payload)
	if err != nil {
		t.Fatalf("ImportLegacy (again): %v", err)
	}
	if again.Completed || again.Variables != 0 {
		t.Errorf("second run = %+v, want nothing done", again)
	}
	state, _ = u.Snapshot(ctx)
	if len(state.Environments[0].Vars) != 2 {
		t.Errorf("the second run duplicated variables: %+v", state.Environments[0].Vars)
	}
}

// A secret keeps its kind and reports that it has no value, so the screen draws it as one to fill
// in rather than as a variable that came over empty.
func TestImportLegacySecretKeepsItsKindWithoutAValue(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()

	payload := `{"environments": [{"id": "env-1", "name": "Local", "vars": [
	  {"id": "v1", "name": "token", "kind": "secret", "enabled": true}
	]}], "globals": []}`

	report, err := u.ImportLegacy(ctx, payload)
	if err != nil {
		t.Fatalf("ImportLegacy: %v", err)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one naming the secret", report.Warnings)
	}
	state, _ := u.Snapshot(ctx)
	v := state.Environments[0].Vars[0]
	if v.Kind != domain.VariableSecret || v.HasValue || v.Value != "" {
		t.Errorf("the variable = %+v, want a secret with no value but its kind", v)
	}
}

func TestImportLegacyWithNoPayloadIsRecorded(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()

	report, err := u.ImportLegacy(ctx, "")
	if err != nil {
		t.Fatalf("ImportLegacy: %v", err)
	}
	if report.Completed {
		t.Error("an empty payload reported a completed import")
	}
	// The claim is finished, so this does not come back on every launch.
	if claimed, _ := u.store.ClaimImport(ctx, LegacySource); claimed {
		t.Error("the import was left pending")
	}
}

func TestEnsureDefaultsSeedsOnlyAnEmptyState(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()

	state, err := u.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	if len(state.Environments) != 1 || state.Environments[0].Name != "Local · dev" {
		t.Fatalf("state = %+v, want the seed environment", state.Environments)
	}
	if vars := state.Environments[0].Vars; len(vars) != 1 || vars[0].Name != "baseUrl" {
		t.Errorf("vars = %+v, want a baseUrl to start from", vars)
	}
	if state.ActiveID != state.Environments[0].ID {
		t.Errorf("activeId = %q, want the seeded environment", state.ActiveID)
	}

	// A second call leaves the user's own setup alone.
	if _, err := u.Create(ctx, EnvironmentDraft{Name: "Prod"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	state, err = u.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("EnsureDefaults (again): %v", err)
	}
	if len(state.Environments) != 2 {
		t.Errorf("environments = %d, want the two that exist already", len(state.Environments))
	}
}

func ptr[T any](v T) *T { return &v }

// Renaming answers exactly as creating does: a name that is empty and one that is too long are two
// different sentences, and the window words them from the code that travels with the refusal. A
// bare sentinel would leave it with the generic "not allowed" for both.
func TestRenameRefusesTheSameNamesCreateDoes(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()

	_, env := seed(t, u, "Local")
	blank := "   "
	long := strings.Repeat("x", maxNameLength+1)

	for _, tc := range []struct {
		name  string
		patch EnvironmentPatch
		want  domain.Code
	}{
		{"blank", EnvironmentPatch{Name: &blank}, domain.CodeNameEmpty},
		{"too long", EnvironmentPatch{Name: &long}, domain.CodeNameTooLong},
	} {
		_, err := u.Update(ctx, env.ID, tc.patch)
		if !errors.Is(err, domain.ErrNotAllowed) {
			t.Errorf("Update with a %s name = %v, want domain.ErrNotAllowed", tc.name, err)
			continue
		}
		if got := domain.CodeOf(err); got != tc.want {
			t.Errorf("Update with a %s name = %q, want %q", tc.name, got, tc.want)
		}
	}

	// And the created case still says what it always said.
	if _, err := u.Create(ctx, EnvironmentDraft{}); domain.CodeOf(err) != domain.CodeNameEmpty {
		t.Errorf("Create(%q) = %q, want %q", "", domain.CodeOf(err), domain.CodeNameEmpty)
	}
}

// A draft carries the colour the environment is drawn with. The create form asks for the name and
// the colour together, and patching what was just made would be a second step the form never shows.
func TestCreateTakesItsColour(t *testing.T) {
	u, _ := newUseCase(t)

	state, err := u.Create(context.Background(), EnvironmentDraft{Name: "Stage", Color: "purple"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := state.Environments[0].Color; got != "purple" {
		t.Errorf("color = %q, want the colour the draft named", got)
	}
	if state.ActiveID != state.Environments[0].ID {
		t.Errorf("activeId = %q, want the environment just made", state.ActiveID)
	}
}

// A copy is the keys a new environment needs and not the values another one holds: the names, the
// kinds and the order come over, and a secret's value does not.
func TestCreateCopiesTheVariablesOfItsBase(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()
	base := baseWithVars(t, u)

	state, err := u.Create(ctx, EnvironmentDraft{Name: "Stage", StartFrom: base.ID})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	made := state.Environments[len(state.Environments)-1]
	if made.ID == base.ID {
		t.Fatal("the new environment is the base itself")
	}
	if len(made.Vars) != 3 {
		t.Fatalf("vars = %d, want the base's three", len(made.Vars))
	}

	names := make([]string, 0, len(made.Vars))
	for i, v := range made.Vars {
		names = append(names, v.Name)
		if v.ID == base.Vars[i].ID {
			t.Errorf("%s kept the base's id, want a variable of this environment's own", v.Name)
		}
		if v.Position != i+1 {
			t.Errorf("%s is at %d, want the base's order kept", v.Name, v.Position)
		}
	}
	if got := strings.Join(names, ", "); got != "baseUrl, token, pageSize" {
		t.Errorf("names = %s, want the base's own list in its own order", got)
	}
	if made.Vars[0].Value != "https://api.example.com" {
		t.Errorf("baseUrl = %q, want an ordinary value copied as it was", made.Vars[0].Value)
	}
}

// The one value a copy does not carry. What a base is for is the list of keys an environment needs;
// a token that two environments silently share is a token nobody remembers changing in both.
func TestACopiedSecretArrivesEmpty(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()
	base := baseWithVars(t, u)

	state, err := u.Create(ctx, EnvironmentDraft{Name: "Stage", StartFrom: base.ID})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	copied := state.Environments[len(state.Environments)-1].Vars[1]
	if copied.Name != "token" || copied.Kind != domain.VariableSecret {
		t.Fatalf("copied = %+v, want the base's secret, still a secret", copied)
	}
	if copied.Value != "" || copied.HasValue {
		t.Errorf("copied secret = %q (hasValue %v), want it empty", copied.Value, copied.HasValue)
	}

	// And the base still holds its own: the copy took nothing from it.
	revealed, err := u.Reveal(ctx, base.Vars[1].ID)
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if revealed != "s3cret" {
		t.Errorf("the base's secret = %q, want it untouched", revealed)
	}
}

// A base nobody can find is refused before anything is written: a draft that names an environment
// and gets an empty one made anyway is a draft whose form lied about what it did.
func TestCreateRefusesABaseThatIsNotThere(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()
	seed(t, u, "Local")

	if _, err := u.Create(ctx, EnvironmentDraft{Name: "Stage", StartFrom: "нет-такого"}); !errors.Is(
		err, domain.ErrNotFound) {
		t.Errorf("Create with an unknown base = %v, want domain.ErrNotFound", err)
	}
	state, err := u.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(state.Environments) != 1 {
		t.Errorf("environments = %d, want nothing made from a base that is not there",
			len(state.Environments))
	}
}

// An environment made from nothing is empty and not a copy of the globals: the two scopes are told
// apart by an empty id, and a branch that read the wrong one would fill every new environment with
// what is only meant to apply everywhere.
func TestCreateWithNoBaseHasNoVariables(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()
	if _, err := u.AddVariable(ctx, domain.EnvScope{}, VariableDraft{
		Name: "locale", Kind: domain.VariableText, Value: "en-US",
	}); err != nil {
		t.Fatalf("AddVariable: %v", err)
	}

	state, err := u.Create(ctx, EnvironmentDraft{Name: "Stage"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := state.Environments[0].Vars; len(got) != 0 {
		t.Errorf("vars = %+v, want none in an environment made from nothing", got)
	}
}

// baseWithVars is the setup the copy tests share: an environment holding the three keys a copy is
// about — an ordinary value, a secret with one, and a second ordinary value.
func baseWithVars(t *testing.T, u *UseCase) domain.Environment {
	t.Helper()
	ctx := context.Background()
	_, base := seed(t, u, "Local")

	for _, draft := range []VariableDraft{
		{Name: "baseUrl", Kind: domain.VariableText, Value: "https://api.example.com"},
		{Name: "token", Kind: domain.VariableSecret, Value: "s3cret"},
		{Name: "pageSize", Kind: domain.VariableText, Value: "25"},
	} {
		if _, err := u.AddVariable(ctx, domain.EnvScope{Environment: base.ID}, draft); err != nil {
			t.Fatalf("AddVariable(%s): %v", draft.Name, err)
		}
	}

	state, err := u.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	return state.Environments[0]
}
