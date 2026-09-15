package environment

import (
	"context"
	"fmt"
	"strings"

	"json-inspector/internal/domain"
	"json-inspector/internal/dotenv"
	"json-inspector/internal/vars"
)

// Variable reads one variable by name in the scope a script names — the active environment, or the
// globals. It is the read behind `pm.environment.get` and `pm.globals.get`.
//
// Unlike the snapshot, it answers with the value, secrets included: a script that signs a request
// needs the token, and this value goes into the sandbox and nowhere else — not into the window, not
// into a log of ours. A script that prints it prints it itself, which is its own business.
func (u *UseCase) Variable(
	ctx context.Context,
	scope domain.VarScope,
	name string,
) (string, bool, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return "", false, err
	}
	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return "", false, err
	}

	switch scope {
	case domain.ScopeEnvironment:
		env, ok := findEnvironment(state, state.ActiveID)
		if !ok {
			// No environment is selected, and that is an answer rather than a mistake: a script
			// asking for a name finds nothing.
			return "", false, nil
		}
		return byName(env.Vars, name)
	case domain.ScopeGlobals:
		return byName(state.Globals, name)
	default:
		return "", false, fmt.Errorf("scope %q: %w", scope, domain.ErrNotAllowed)
	}
}

// SetVariable writes a variable by name, adding it when nobody has that name yet. Adding is the
// point: a script that records a token for the next request of a run must not have to know whether
// that variable exists, and the sheet is where names are looked at, not here.
func (u *UseCase) SetVariable(
	ctx context.Context,
	scope domain.VarScope,
	name string,
	value string,
) error {
	name = strings.TrimSpace(name)
	// A name no `{{token}}` can carry is a row nobody can use: the script would be writing into a
	// place the request it is running around cannot read.
	if name == "" || !vars.ValidName(name) {
		return fmt.Errorf("variable name %q: %w", name, domain.ErrNotAllowed)
	}

	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return err
	}

	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return err
	}

	where := domain.EnvScope{}
	switch scope {
	case domain.ScopeEnvironment:
		if _, ok := findEnvironment(state, state.ActiveID); !ok {
			return domain.Refuse(domain.CodeNoEnvironment, domain.ErrNotAllowed, nil)
		}
		where.Environment = state.ActiveID
	case domain.ScopeGlobals:
	default:
		return fmt.Errorf("scope %q: %w", scope, domain.ErrNotAllowed)
	}

	if existing, ok := existing(scopeVariables(state, where), name); ok {
		existing.Value = value
		return u.store.SaveVariable(ctx, workspace, where, existing)
	}

	position, err := nextVariablePosition(state, where)
	if err != nil {
		return err
	}
	return u.store.SaveVariable(ctx, workspace, where, domain.Variable{
		ID:       u.ids(),
		Name:     name,
		Value:    value,
		Kind:     domain.VariableText,
		Enabled:  true,
		Position: position,
	})
}

func byName(variables []domain.Variable, name string) (string, bool, error) {
	for _, v := range variables {
		if v.Name == name {
			return v.Value, true, nil
		}
	}
	return "", false, nil
}

func existing(variables []domain.Variable, name string) (domain.Variable, bool) {
	for _, v := range variables {
		if v.Name == name {
			return v, true
		}
	}
	return domain.Variable{}, false
}

// A secret leaves this side masked — see hideSecretValues.

// VariableDraft is a variable on its way in: the sheet's "add row" leaves it empty, the .env
// dialog fills it in, and both go through one call.
type VariableDraft struct {
	Name  string              `json:"name"`
	Kind  domain.VariableKind `json:"kind"`
	Value string              `json:"value,omitempty"`
}

// AddVariable appends a variable to a scope.
func (u *UseCase) AddVariable(
	ctx context.Context,
	scope domain.EnvScope,
	draft VariableDraft,
) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return domain.EnvState{}, err
	}
	position, err := nextVariablePosition(state, scope)
	if err != nil {
		return domain.EnvState{}, err
	}

	name := strings.TrimSpace(draft.Name)
	if name != "" && !vars.ValidName(name) {
		return domain.EnvState{}, fmt.Errorf("variable name: %w", domain.ErrNotAllowed)
	}
	if draft.Kind != domain.VariableSecret {
		draft.Kind = domain.VariableText
	}

	v := domain.Variable{
		ID:       u.ids(),
		Name:     name,
		Value:    draft.Value,
		Kind:     draft.Kind,
		Enabled:  true,
		Position: position,
	}
	if err := u.store.SaveVariable(ctx, workspace, scope, v); err != nil {
		return domain.EnvState{}, err
	}
	return u.snapshot(ctx, workspace)
}

// VariablePatch edits one variable. SetValue is what keeps a secret's value when only its name or
// its enabled flag is being changed: the sheet never has the old value to send back.
type VariablePatch struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Kind     domain.VariableKind `json:"kind"`
	Enabled  bool                `json:"enabled"`
	Value    string              `json:"value,omitempty"`
	SetValue bool                `json:"setValue"`
}

func (u *UseCase) UpdateVariable(
	ctx context.Context,
	scope domain.EnvScope,
	patch VariablePatch,
) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return domain.EnvState{}, err
	}
	existing, ok := findVariable(state, scope, patch.ID)
	if !ok {
		return domain.EnvState{}, fmt.Errorf("variable %s: %w", patch.ID, domain.ErrNotFound)
	}

	name := strings.TrimSpace(patch.Name)
	if name != "" && !vars.ValidName(name) {
		return domain.EnvState{}, fmt.Errorf("variable name: %w", domain.ErrNotAllowed)
	}

	value := existing.Value
	if patch.SetValue {
		value = patch.Value
	}
	kind := patch.Kind
	if kind != domain.VariableSecret {
		kind = domain.VariableText
	}

	updated := domain.Variable{
		ID:       patch.ID,
		Name:     name,
		Value:    value,
		Kind:     kind,
		Enabled:  patch.Enabled,
		Position: existing.Position,
	}
	if err := u.store.SaveVariable(ctx, workspace, scope, updated); err != nil {
		return domain.EnvState{}, err
	}
	return u.snapshot(ctx, workspace)
}

func (u *UseCase) RemoveVariable(
	ctx context.Context,
	scope domain.EnvScope,
	id string,
) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return domain.EnvState{}, err
	}
	if _, ok := findVariable(state, scope, id); !ok {
		return domain.EnvState{}, fmt.Errorf("variable %s: %w", id, domain.ErrNotFound)
	}
	if err := u.store.DeleteVariable(ctx, workspace, id); err != nil {
		return domain.EnvState{}, err
	}
	return u.snapshot(ctx, workspace)
}

// ImportEntries merges a parsed .env file into a scope: a name that is already there is replaced,
// one that is not is appended. It is the same rule the import dialog offers, without the question.
func (u *UseCase) ImportEntries(
	ctx context.Context,
	scope domain.EnvScope,
	entries []dotenv.Entry,
) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return domain.EnvState{}, err
	}
	position, err := nextVariablePosition(state, scope)
	if err != nil {
		return domain.EnvState{}, err
	}

	existing := map[string]domain.Variable{}
	for _, v := range scopeVariables(state, scope) {
		existing[v.Name] = v
	}

	for _, entry := range entries {
		kind := domain.VariableText
		if entry.Secret {
			kind = domain.VariableSecret
		}

		v, seen := existing[entry.Name]
		if seen {
			v.Value = entry.Value
			v.Kind = kind
			v.Enabled = true
		} else {
			v = domain.Variable{
				ID:       u.ids(),
				Name:     entry.Name,
				Value:    entry.Value,
				Kind:     kind,
				Enabled:  true,
				Position: position,
			}
			position++
		}
		if err := u.store.SaveVariable(ctx, workspace, scope, v); err != nil {
			return domain.EnvState{}, err
		}
	}
	return u.snapshot(ctx, workspace)
}

// Reveal is the deliberate "show me" behind the sheet's eye button, and the only way a secret's
// value leaves the database.
func (u *UseCase) Reveal(ctx context.Context, id string) (string, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return "", err
	}
	return u.store.VariableValue(ctx, workspace, id)
}

// scopeVariables lists what a scope holds right now; an empty scope is the globals.
func scopeVariables(state domain.EnvState, scope domain.EnvScope) []domain.Variable {
	if scope.Environment == "" {
		return state.Globals
	}
	for _, env := range state.Environments {
		if env.ID == scope.Environment {
			return env.Vars
		}
	}
	return nil
}

func nextVariablePosition(state domain.EnvState, scope domain.EnvScope) (int, error) {
	if scope.Environment != "" {
		if _, ok := findEnvironment(state, scope.Environment); !ok {
			return 0, fmt.Errorf("environment %s: %w", scope.Environment, domain.ErrNotFound)
		}
	}
	position := 0
	for _, v := range scopeVariables(state, scope) {
		if v.Position >= position {
			position = v.Position + 1
		}
	}
	return position, nil
}

// hideSecretValues copies the state without the secrets' values. The variables themselves stay: the
// sheet lists them, shows their kind and needs their ids to reveal one.
//
// The parameter is named list rather than vars because `vars` is the package that checks a name,
// and a parameter of that name would shadow it inside the one function that might want it.
func hideSecretValues(state domain.EnvState) domain.EnvState {
	hide := func(list []domain.Variable) []domain.Variable {
		out := make([]domain.Variable, len(list))
		for i, v := range list {
			if v.Kind == domain.VariableSecret {
				v.HasValue = v.Value != ""
				v.Value = ""
			}
			out[i] = v
		}
		return out
	}

	for i := range state.Environments {
		state.Environments[i].Vars = hide(state.Environments[i].Vars)
	}
	state.Globals = hide(state.Globals)
	return state
}
