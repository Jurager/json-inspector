package environment

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
)

// A script reads a variable by name in one scope, and the snapshot's rules do not apply to it: a
// secret is read as its value, because signing a request with it is the whole point of having one.
func TestVariableReadsByName(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()

	_, env := seed(t, u, "Local · dev")
	if _, err := u.AddVariable(ctx, domain.EnvScope{Environment: env.ID}, VariableDraft{
		Name: "baseUrl", Value: "https://api.example.com",
	}); err != nil {
		t.Fatalf("AddVariable: %v", err)
	}
	if _, err := u.AddVariable(ctx, domain.EnvScope{}, VariableDraft{
		Name: "token", Value: "s3cret", Kind: domain.VariableSecret,
	}); err != nil {
		t.Fatalf("AddVariable: %v", err)
	}

	value, found, err := u.Variable(ctx, domain.ScopeEnvironment, "baseUrl")
	if err != nil || !found || value != "https://api.example.com" {
		t.Errorf("baseUrl = %q, %v, %v, want the environment's value", value, found, err)
	}
	value, found, err = u.Variable(ctx, domain.ScopeGlobals, "token")
	if err != nil || !found || value != "s3cret" {
		t.Errorf("token = %q, %v, %v, want the secret's value", value, found, err)
	}

	// Each scope answers for itself: the globals have no baseUrl, and the environment has no token.
	if _, found, err := u.Variable(ctx, domain.ScopeGlobals, "baseUrl"); err != nil || found {
		t.Errorf("the globals answered with a variable of the environment: %v, %v", found, err)
	}
	if _, found, err := u.Variable(ctx, domain.ScopeEnvironment, "token"); err != nil || found {
		t.Errorf("the environment answered with a global: %v, %v", found, err)
	}
	if _, found, err := u.Variable(ctx, domain.ScopeEnvironment, "нет-такой"); err != nil || found {
		t.Errorf("an unknown name = %v, %v, want nothing", found, err)
	}
}

// A script writes by name, and a name nobody has is added: a script recording a token for the next
// request of a run cannot be expected to have created the variable first.
func TestSetVariableAddsAndReplaces(t *testing.T) {
	u, store := newUseCase(t)
	ctx := context.Background()

	seed(t, u, "Local · dev")
	if err := u.SetVariable(ctx, domain.ScopeEnvironment, "page", "2"); err != nil {
		t.Fatalf("SetVariable: %v", err)
	}
	if err := u.SetVariable(ctx, domain.ScopeGlobals, "auth", "из-скрипта"); err != nil {
		t.Fatalf("SetVariable: %v", err)
	}

	value, found, err := u.Variable(ctx, domain.ScopeEnvironment, "page")
	if err != nil || !found || value != "2" {
		t.Errorf("page = %q, %v, %v, want what the script wrote", value, found, err)
	}
	if value, _, _ := u.Variable(ctx, domain.ScopeGlobals, "auth"); value != "из-скрипта" {
		t.Errorf("auth = %q, want what the script wrote", value)
	}

	// Writing the same name again replaces it in place: two rows with one name in one scope is a
	// scope that answers differently depending on the order it is read in.
	before, err := store.EnvState(ctx)
	if err != nil {
		t.Fatalf("EnvState: %v", err)
	}
	was := placeOf(before.Environments[0].Vars, "page")

	if err := u.SetVariable(ctx, domain.ScopeEnvironment, "page", "3"); err != nil {
		t.Fatalf("SetVariable: %v", err)
	}
	after, err := store.EnvState(ctx)
	if err != nil {
		t.Fatalf("EnvState: %v", err)
	}
	if count := countOf(after.Environments[0].Vars, "page"); count != 1 {
		t.Errorf("page appears %d time(s), want once", count)
	}
	if place := placeOf(after.Environments[0].Vars, "page"); place != was {
		t.Errorf("page moved from %d to %d, want it where it was", was, place)
	}
	if value, _, _ := u.Variable(ctx, domain.ScopeEnvironment, "page"); value != "3" {
		t.Errorf("page = %q, want the second value", value)
	}
}

func countOf(variables []domain.Variable, name string) int {
	count := 0
	for _, v := range variables {
		if v.Name == name {
			count++
		}
	}
	return count
}

func placeOf(variables []domain.Variable, name string) int {
	for _, v := range variables {
		if v.Name == name {
			return v.Position
		}
	}
	return -1
}

// What a script cannot do, it says so about: there is no environment selected to write into, there is
// no scope called that, and a name no `{{token}}` can carry is a name nobody can read back.
func TestVariableRefusesWhatItCannotDo(t *testing.T) {
	u, _ := newUseCase(t)
	ctx := context.Background()

	// No environment exists at all, so none is active.
	if _, found, err := u.Variable(ctx, domain.ScopeEnvironment, "baseUrl"); err != nil || found {
		t.Errorf("a name in a scope that is not selected = %v, %v, want nothing", found, err)
	}
	if err := u.SetVariable(ctx, domain.ScopeEnvironment, "token", "1"); err == nil {
		t.Error("writing to an environment nobody selected was allowed")
	}

	for _, scope := range []domain.VarScope{domain.ScopeRun, "какая-то"} {
		if _, _, err := u.Variable(ctx, scope, "token"); !errors.Is(err, domain.ErrNotAllowed) {
			t.Errorf("reading %q = %v, want ErrNotAllowed", scope, err)
		}
		if err := u.SetVariable(ctx, scope, "token", "1"); !errors.Is(err, domain.ErrNotAllowed) {
			t.Errorf("writing to %q = %v, want ErrNotAllowed", scope, err)
		}
	}

	for _, name := range []string{"", "  ", "a b", "{{x}}"} {
		if err := u.SetVariable(ctx, domain.ScopeGlobals, name, "1"); !errors.Is(err, domain.ErrNotAllowed) {
			t.Errorf("writing %q = %v, want ErrNotAllowed", name, err)
		}
	}
}
