// Package environment owns the environments and their variables: what is stored, which scope wins
// when a name exists twice, and what a `{{token}}` resolves to.
package environment

import (
	"context"
	"fmt"
	"json-inspector/internal/domain"
	"json-inspector/internal/dotenv"
	"json-inspector/internal/platform"
	"json-inspector/internal/vars"
	"strconv"
	"strings"
)

// maxNameLength is the design's limit for an environment's name: long enough for "Prod · EU-West",
// short enough to stay one chip in the titlebar.
const maxNameLength = 40

type UseCase struct {
	store Store
	scope Scope
	ids   platform.IDGen
}

func NewUseCase(store Store, scope Scope, ids platform.IDGen) *UseCase {
	return &UseCase{store: store, scope: scope, ids: ids}
}

// Snapshot is the whole screen. Secret values are withheld: a variable that has one says so, and
// Reveal is what shows it.
//
// The workspace is resolved once, at the door, and carried as a value from there on: an operation
// that read the pointer again halfway through would write half of itself into the workspace the
// user has just switched away from.
func (u *UseCase) Snapshot(ctx context.Context) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}
	return u.snapshot(ctx, workspace)
}

func (u *UseCase) snapshot(ctx context.Context, workspace string) (domain.EnvState, error) {
	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return domain.EnvState{}, err
	}
	return hideSecretValues(state), nil
}

// Patch is a partial update: a nil field is left as it is.
type Patch struct {
	Name     *string `json:"name,omitempty"`
	Color    *string `json:"color,omitempty"`
	Readonly *bool   `json:"readonly,omitempty"`
}

// Create adds an environment at the end of the list. It becomes the active one: a new environment
// is being set up, and editing the variables of one you are not in is how mistakes happen.
func (u *UseCase) Create(ctx context.Context, name string) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return domain.EnvState{}, domain.Refuse(domain.CodeNameEmpty, domain.ErrNotAllowed, nil)
	}
	if len([]rune(name)) > maxNameLength {
		return domain.EnvState{}, domain.Refuse(domain.CodeNameTooLong, domain.ErrNotAllowed,
			domain.Args{"max": strconv.Itoa(maxNameLength)})
	}

	current, err := u.store.EnvState(ctx, workspace)
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
	if err := u.store.SaveEnvironment(ctx, workspace, env); err != nil {
		return domain.EnvState{}, err
	}
	if err := u.store.SetActiveEnvironment(ctx, workspace, env.ID); err != nil {
		return domain.EnvState{}, err
	}
	return u.snapshot(ctx, workspace)
}

// Update applies a patch. The readonly flag is what the design calls "Prod": it locks the
// variables until the user unlocks them for this session.
func (u *UseCase) Update(ctx context.Context, id string, patch Patch) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return domain.EnvState{}, err
	}
	env, ok := findEnvironment(state, id)
	if !ok {
		return domain.EnvState{}, fmt.Errorf("environment %s: %w", id, domain.ErrNotFound)
	}

	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" || len([]rune(name)) > maxNameLength {
			return domain.EnvState{}, fmt.Errorf("environment name: %w", domain.ErrNotAllowed)
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
	if err := u.store.SaveEnvironment(ctx, workspace, env); err != nil {
		return domain.EnvState{}, err
	}
	return u.snapshot(ctx, workspace)
}

// Delete removes an environment and everything in it. If it was the active one, the selection
// falls back to no environment rather than to a sibling: resolving against an environment the user
// did not choose would be worse than resolving against nothing.
func (u *UseCase) Delete(ctx context.Context, id string) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return domain.EnvState{}, err
	}
	if _, ok := findEnvironment(state, id); !ok {
		return domain.EnvState{}, fmt.Errorf("environment %s: %w", id, domain.ErrNotFound)
	}
	if err := u.store.DeleteEnvironment(ctx, workspace, id); err != nil {
		return domain.EnvState{}, err
	}
	if state.ActiveID == id {
		if err := u.store.SetActiveEnvironment(ctx, workspace, ""); err != nil {
			return domain.EnvState{}, err
		}
	}
	return u.snapshot(ctx, workspace)
}

// Activate selects the environment the request preview resolves against.
func (u *UseCase) Activate(ctx context.Context, id string) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}
	if id != "" {
		state, err := u.store.EnvState(ctx, workspace)
		if err != nil {
			return domain.EnvState{}, err
		}
		if _, ok := findEnvironment(state, id); !ok {
			return domain.EnvState{}, fmt.Errorf("environment %s: %w", id, domain.ErrNotFound)
		}
	}
	if err := u.store.SetActiveEnvironment(ctx, workspace, id); err != nil {
		return domain.EnvState{}, err
	}
	return u.snapshot(ctx, workspace)
}

// VariableDraft is a variable on its way in: the sheet's "add row" leaves it empty, the .env
// dialog fills it in, and both go through one call.
type VariableDraft struct {
	Name  string              `json:"name"`
	Kind  domain.VariableKind `json:"kind"`
	Value string              `json:"value,omitempty"`
}

// AddVariable appends a variable to a scope.
func (u *UseCase) AddVariable(ctx context.Context, scope domain.EnvScope, draft VariableDraft) (domain.EnvState, error) {
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

// EnsureDefaults gives a workspace the one environment the app has always started with, so a space
// the user has just made has somewhere to type a base URL instead of an empty screen. It only ever
// acts on an empty state: as soon as anything exists, the user's own setup is the answer.
func (u *UseCase) EnsureDefaults(ctx context.Context) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return domain.EnvState{}, err
	}
	if len(state.Environments) > 0 || len(state.Globals) > 0 {
		return u.snapshot(ctx, workspace)
	}

	env := domain.Environment{ID: u.ids(), Name: "Local · dev", Position: 1}
	if err := u.store.SaveEnvironment(ctx, workspace, env); err != nil {
		return domain.EnvState{}, err
	}
	baseURL := domain.Variable{
		ID:      u.ids(),
		Name:    "baseUrl",
		Value:   "http://localhost:8000",
		Kind:    domain.VariableText,
		Enabled: true,
		// One-based to match the environment above: positions are ordered, not indexed.
		Position: 1,
	}
	if err := u.store.SaveVariable(ctx, workspace, domain.EnvScope{Environment: env.ID}, baseURL); err != nil {
		return domain.EnvState{}, err
	}
	if err := u.store.SetActiveEnvironment(ctx, workspace, env.ID); err != nil {
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

func (u *UseCase) UpdateVariable(ctx context.Context, scope domain.EnvScope, patch VariablePatch) (domain.EnvState, error) {
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

func (u *UseCase) RemoveVariable(ctx context.Context, scope domain.EnvScope, id string) (domain.EnvState, error) {
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
func (u *UseCase) ImportEntries(ctx context.Context, scope domain.EnvScope, entries []dotenv.Entry) (domain.EnvState, error) {
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
