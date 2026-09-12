// Package environment owns the environments and their variables: what is stored, which scope wins
// when a name exists twice, and what a `{{token}}` resolves to.
package environment

import (
	"context"
	"fmt"
	"strings"

	"json-inspector/internal/domain"
	"json-inspector/internal/dotenv"
	"json-inspector/internal/platform"
	"json-inspector/internal/vars"
)

// maxNameLength is the design's limit for an environment's name: long enough for "Prod · EU-West",
// short enough to stay one chip in the titlebar.
const maxNameLength = 40

type UseCase struct {
	store Store
	ids   platform.IDGen
}

func NewUseCase(store Store, ids platform.IDGen) *UseCase {
	return &UseCase{store: store, ids: ids}
}

// Snapshot is the whole screen. Secret values are withheld: a variable that has one says so, and
// Reveal is what shows it.
func (u *UseCase) Snapshot(ctx context.Context) (domain.EnvState, error) {
	state, err := u.store.EnvState(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}
	return hideSecretValues(state), nil
}

// Patch is a partial update: a nil field is left as it is.
type Patch struct {
	Name     *string
	Color    *string
	Readonly *bool
}

// Create adds an environment at the end of the list. It becomes the active one: a new environment
// is being set up, and editing the variables of one you are not in is how mistakes happen.
func (u *UseCase) Create(ctx context.Context, name string) (domain.EnvState, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.EnvState{}, fmt.Errorf("имя окружения не может быть пустым: %w", domain.ErrNotAllowed)
	}
	if len([]rune(name)) > maxNameLength {
		return domain.EnvState{}, fmt.Errorf("имя длиннее %d символов: %w", maxNameLength, domain.ErrNotAllowed)
	}

	current, err := u.store.EnvState(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	position := 0
	for _, env := range current.Environments {
		if env.Position >= position {
			position = env.Position + 1
		}
	}
	env := domain.Environment{ID: u.ids(), Name: name, Position: position}
	if err := u.store.SaveEnvironment(ctx, env); err != nil {
		return domain.EnvState{}, err
	}
	if err := u.store.SetActiveEnvironment(ctx, env.ID); err != nil {
		return domain.EnvState{}, err
	}
	return u.Snapshot(ctx)
}

// Update applies a patch. The readonly flag is what the design calls "Prod": it locks the
// variables until the user unlocks them for this session.
func (u *UseCase) Update(ctx context.Context, id string, patch Patch) (domain.EnvState, error) {
	state, err := u.store.EnvState(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}
	env, ok := findEnvironment(state, id)
	if !ok {
		return domain.EnvState{}, fmt.Errorf("окружение %s: %w", id, domain.ErrNotFound)
	}

	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" || len([]rune(name)) > maxNameLength {
			return domain.EnvState{}, fmt.Errorf("имя окружения: %w", domain.ErrNotAllowed)
		}
		env.Name = name
	}
	if patch.Color != nil {
		env.Color = *patch.Color
	}
	if patch.Readonly != nil {
		env.Readonly = *patch.Readonly
	}

	// Vars are written by their own calls; SaveEnvironment only needs the environment itself.
	env.Vars = nil
	if err := u.store.SaveEnvironment(ctx, env); err != nil {
		return domain.EnvState{}, err
	}
	return u.Snapshot(ctx)
}

// Delete removes an environment and everything in it. If it was the active one, the selection
// falls back to no environment rather than to a sibling: resolving against an environment the user
// did not choose would be worse than resolving against nothing.
func (u *UseCase) Delete(ctx context.Context, id string) (domain.EnvState, error) {
	state, err := u.store.EnvState(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}
	if _, ok := findEnvironment(state, id); !ok {
		return domain.EnvState{}, fmt.Errorf("окружение %s: %w", id, domain.ErrNotFound)
	}
	if err := u.store.DeleteEnvironment(ctx, id); err != nil {
		return domain.EnvState{}, err
	}
	if state.ActiveID == id {
		if err := u.store.SetActiveEnvironment(ctx, ""); err != nil {
			return domain.EnvState{}, err
		}
	}
	return u.Snapshot(ctx)
}

// Activate selects the environment the request preview resolves against.
func (u *UseCase) Activate(ctx context.Context, id string) (domain.EnvState, error) {
	if id != "" {
		state, err := u.store.EnvState(ctx)
		if err != nil {
			return domain.EnvState{}, err
		}
		if _, ok := findEnvironment(state, id); !ok {
			return domain.EnvState{}, fmt.Errorf("окружение %s: %w", id, domain.ErrNotFound)
		}
	}
	if err := u.store.SetActiveEnvironment(ctx, id); err != nil {
		return domain.EnvState{}, err
	}
	return u.Snapshot(ctx)
}

// AddVariable appends an empty variable to a scope, which is what the sheet's "add row" needs.
func (u *UseCase) AddVariable(ctx context.Context, scope domain.EnvScope, kind domain.VariableKind) (domain.EnvState, error) {
	state, err := u.store.EnvState(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}
	position, err := nextVariablePosition(state, scope)
	if err != nil {
		return domain.EnvState{}, err
	}
	if kind != domain.VariableSecret {
		kind = domain.VariableText
	}

	v := domain.Variable{
		ID:       u.ids(),
		Name:     "",
		Kind:     kind,
		Enabled:  true,
		Position: position,
	}
	if err := u.store.SaveVariable(ctx, scope, v); err != nil {
		return domain.EnvState{}, err
	}
	return u.Snapshot(ctx)
}

// VariablePatch edits one variable. SetValue is what keeps a secret's value when only its name or
// its enabled flag is being changed: the sheet never has the old value to send back.
type VariablePatch struct {
	ID       string
	Name     string
	Kind     domain.VariableKind
	Enabled  bool
	Value    string
	SetValue bool
}

func (u *UseCase) UpdateVariable(ctx context.Context, scope domain.EnvScope, patch VariablePatch) (domain.EnvState, error) {
	state, err := u.store.EnvState(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}
	existing, ok := findVariable(state, scope, patch.ID)
	if !ok {
		return domain.EnvState{}, fmt.Errorf("переменная %s: %w", patch.ID, domain.ErrNotFound)
	}

	name := strings.TrimSpace(patch.Name)
	if name != "" && !vars.ValidName(name) {
		return domain.EnvState{}, fmt.Errorf("имя переменной: %w", domain.ErrNotAllowed)
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
	if err := u.store.SaveVariable(ctx, scope, updated); err != nil {
		return domain.EnvState{}, err
	}
	return u.Snapshot(ctx)
}

func (u *UseCase) RemoveVariable(ctx context.Context, scope domain.EnvScope, id string) (domain.EnvState, error) {
	state, err := u.store.EnvState(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}
	if _, ok := findVariable(state, scope, id); !ok {
		return domain.EnvState{}, fmt.Errorf("переменная %s: %w", id, domain.ErrNotFound)
	}
	if err := u.store.DeleteVariable(ctx, id); err != nil {
		return domain.EnvState{}, err
	}
	return u.Snapshot(ctx)
}

// ImportEntries merges a parsed .env file into a scope: a name that is already there is replaced,
// one that is not is appended. It is the same rule the import dialog offers, without the question.
func (u *UseCase) ImportEntries(ctx context.Context, scope domain.EnvScope, entries []dotenv.Entry) (domain.EnvState, error) {
	state, err := u.store.EnvState(ctx)
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
		if err := u.store.SaveVariable(ctx, scope, v); err != nil {
			return domain.EnvState{}, err
		}
	}
	return u.Snapshot(ctx)
}

// Reveal is the deliberate "show me" behind the sheet's eye button, and the only way a secret's
// value leaves the database.
func (u *UseCase) Reveal(ctx context.Context, id string) (string, error) {
	return u.store.VariableValue(ctx, id)
}

func findEnvironment(state domain.EnvState, id string) (domain.Environment, bool) {
	for _, env := range state.Environments {
		if env.ID == id {
			return env, true
		}
	}
	return domain.Environment{}, false
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
			return 0, fmt.Errorf("окружение %s: %w", scope.Environment, domain.ErrNotFound)
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

// hideSecretValues copies the state without the secrets' values. The variables themselves stay:
// the sheet lists them, shows their kind and needs their ids to reveal one.
func hideSecretValues(state domain.EnvState) domain.EnvState {
	hide := func(vars []domain.Variable) []domain.Variable {
		out := make([]domain.Variable, len(vars))
		for i, v := range vars {
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
