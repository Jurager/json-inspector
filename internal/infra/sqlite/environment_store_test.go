package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

func TestEnvStateRoundTrip(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	env := domain.Environment{ID: "env-1", Name: "Local", Color: "green", Position: 1}
	if err := store.SaveEnvironment(ctx, ws, env); err != nil {
		t.Fatalf("SaveEnvironment: %v", err)
	}
	if err := store.SaveEnvironment(ctx, ws,
		domain.Environment{ID: "env-2", Name: "Prod", Readonly: true, Position: 2}); err != nil {
		t.Fatalf("SaveEnvironment: %v", err)
	}

	if err := store.SaveVariable(ctx, ws, domain.EnvScope{Environment: "env-1"}, domain.Variable{
		ID: "v1", Name: "base_url", Value: "https://api.example.com", Kind: domain.VariableText,
		Enabled: true, Position: 1,
	}); err != nil {
		t.Fatalf("SaveVariable: %v", err)
	}
	if err := store.SaveVariable(ctx, ws, domain.EnvScope{Environment: "env-1"}, domain.Variable{
		ID: "v2", Name: "token", Value: "s3cret", Kind: domain.VariableSecret, Enabled: true, Position: 2,
	}); err != nil {
		t.Fatalf("SaveVariable: %v", err)
	}
	if err := store.SaveVariable(ctx, ws, domain.EnvScope{}, domain.Variable{
		ID: "g1", Name: "page_size", Value: "10", Kind: domain.VariableText, Enabled: true, Position: 1,
	}); err != nil {
		t.Fatalf("SaveVariable (globals): %v", err)
	}
	if err := store.SetActiveEnvironment(ctx, ws, "env-1"); err != nil {
		t.Fatalf("SetActiveEnvironment: %v", err)
	}

	state, err := store.EnvState(ctx, ws)
	if err != nil {
		t.Fatalf("EnvState: %v", err)
	}
	if state.ActiveID != "env-1" {
		t.Errorf("activeId = %q, want env-1", state.ActiveID)
	}
	if len(state.Environments) != 2 {
		t.Fatalf("environments = %d, want 2", len(state.Environments))
	}
	if state.Environments[0].Name != "Local" || state.Environments[1].Name != "Prod" {
		t.Errorf("environments came back in the wrong order: %v", state.Environments)
	}
	if !state.Environments[1].Readonly {
		t.Error("Prod lost its readonly flag")
	}
	local := state.Environments[0]
	if len(local.Vars) != 2 || local.Vars[0].Name != "base_url" || local.Vars[1].Name != "token" {
		t.Errorf("variables = %v, want base_url then token", local.Vars)
	}
	if local.Vars[1].Kind != domain.VariableSecret || !local.Vars[1].HasValue {
		t.Errorf("the secret lost its kind or its value: %+v", local.Vars[1])
	}
	if len(state.Globals) != 1 || state.Globals[0].Name != "page_size" {
		t.Errorf("globals = %v, want one row", state.Globals)
	}
}

func TestVariablesMigrateBetweenScopes(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	// The same name may exist in an environment and in the globals: that is how an override works.
	for _, scope := range []domain.EnvScope{{Environment: "env-1"}, {}} {
		v := domain.Variable{ID: "v-" + scope.Environment, Name: "token", Value: "x",
			Kind: domain.VariableText, Enabled: true}
		if err := store.SaveVariable(ctx, ws, scope, v); err != nil {
			t.Fatalf("SaveVariable in %q: %v", scope.Environment, err)
		}
	}

	// And a second one under the same scope is a conflict rather than a database error.
	err := store.SaveVariable(ctx, ws, domain.EnvScope{Environment: "env-1"}, domain.Variable{
		ID: "other", Name: "token", Kind: domain.VariableText, Enabled: true,
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate name = %v, want domain.ErrConflict", err)
	}

	// The same variable, renamed, keeps its row: an update is not a conflict with itself.
	if err := store.SaveVariable(ctx, ws, domain.EnvScope{Environment: "env-1"}, domain.Variable{
		ID: "v-env-1", Name: "api_token", Kind: domain.VariableSecret, Value: "s", Enabled: true,
	}); err != nil {
		t.Errorf("renaming a variable failed: %v", err)
	}
	if value, err := store.VariableValue(ctx, ws, "v-env-1"); err != nil || value != "s" {
		t.Errorf("after the rename: value = %q, err = %v; want the row updated in place", value, err)
	}
}

func TestDeleteEnvironmentTakesItsVariables(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveEnvironment(ctx, ws,
		domain.Environment{ID: "env-1", Name: "Local"}); err != nil {
		t.Fatalf("SaveEnvironment: %v", err)
	}
	if err := store.SaveVariable(ctx, ws, domain.EnvScope{Environment: "env-1"}, domain.Variable{
		ID: "v1", Name: "base_url", Kind: domain.VariableText, Enabled: true,
	}); err != nil {
		t.Fatalf("SaveVariable: %v", err)
	}
	if err := store.SaveVariable(ctx, ws, domain.EnvScope{}, domain.Variable{
		ID: "g1", Name: "page_size", Kind: domain.VariableText, Enabled: true,
	}); err != nil {
		t.Fatalf("SaveVariable (globals): %v", err)
	}

	if err := store.DeleteEnvironment(ctx, ws, "env-1"); err != nil {
		t.Fatalf("DeleteEnvironment: %v", err)
	}

	state, err := store.EnvState(ctx, ws)
	if err != nil {
		t.Fatalf("EnvState: %v", err)
	}
	if len(state.Environments) != 0 {
		t.Errorf("environments = %v, want none", state.Environments)
	}
	if len(state.Globals) != 1 {
		t.Errorf("globals = %v, want the global to survive the environment", state.Globals)
	}
	if _, err := store.VariableValue(ctx, ws, "v1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the environment's variable outlived it: %v", err)
	}
}

func TestVariableValueAndMissing(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveVariable(ctx, ws, domain.EnvScope{}, domain.Variable{
		ID: "v1", Name: "token", Value: "s3cret", Kind: domain.VariableSecret, Enabled: true,
	}); err != nil {
		t.Fatalf("SaveVariable: %v", err)
	}

	value, err := store.VariableValue(ctx, ws, "v1")
	if err != nil || value != "s3cret" {
		t.Errorf("VariableValue = %q, %v; want the stored value", value, err)
	}
	if _, err := store.VariableValue(ctx, ws, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown id = %v, want domain.ErrNotFound", err)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if _, ok, err := store.Setting(ctx, domain.SettingActiveWorkspace); err != nil || ok {
		t.Errorf("Setting on an empty table = ok:%v err:%v, want absent", ok, err)
	}
	if err := store.SaveSetting(ctx, "theme", "dark"); err != nil {
		t.Fatalf("SaveSetting: %v", err)
	}
	if err := store.SaveSetting(ctx, "theme", "light"); err != nil {
		t.Fatalf("SaveSetting (overwrite): %v", err)
	}

	all, err := store.Settings(ctx)
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if all["theme"] != "light" {
		t.Errorf("theme = %q, want the overwritten value", all["theme"])
	}
}

func TestStorePathIsInsideTheDataDir(t *testing.T) {
	dir := platform.DataDir(t.TempDir())
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if filepath.Dir(store.Path()) != string(dir) {
		t.Errorf("database at %q, want it inside %q", store.Path(), dir)
	}
}
