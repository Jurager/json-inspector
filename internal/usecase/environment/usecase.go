// Package environment owns the environments and the variables that resolve against them: what a
// request interpolates, in which scope, and which of the values are secrets.
package environment

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
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

// EnvironmentPatch is a partial update: a nil field is left as it is.
type EnvironmentPatch struct {
	Name     *string `json:"name,omitempty"`
	Color    *string `json:"color,omitempty"`
	Readonly *bool   `json:"readonly,omitempty"`
}

// EnvironmentDraft is what a new environment is asked for: its name, the colour its dot and avatar
// are drawn with, and — when it is made from an existing one — the environment it starts as a copy
// of. A draft rather than three more parameters: the create form asks for all three at once, and a
// signature that grows an argument every time the form gains a field is one nobody can read.
type EnvironmentDraft struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
	// StartFrom is an environment id, empty for a blank one.
	StartFrom string `json:"startFrom,omitempty"`
}

// Create adds an environment at the end of the list. It becomes the active one: a new environment
// is being set up, and editing the variables of one you are not in is how mistakes happen.
//
// A draft that names a base is filled from it — the same keys, kinds and order — so that a stage
// environment starts as a copy of dev rather than as a blank page. A secret is copied without its
// value: what a copy is for is the set of keys an environment needs, and a value that two
// environments silently share is a value nobody remembers changing in both.
func (u *UseCase) Create(ctx context.Context, draft EnvironmentDraft) (domain.EnvState, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.EnvState{}, err
	}

	name, err := validEnvironmentName(draft.Name)
	if err != nil {
		return domain.EnvState{}, err
	}

	current, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return domain.EnvState{}, err
	}

	base := domain.Environment{}
	if draft.StartFrom != "" {
		found, ok := findEnvironment(current, draft.StartFrom)
		if !ok {
			return domain.EnvState{}, fmt.Errorf("environment %s: %w", draft.StartFrom,
				domain.ErrNotFound)
		}
		base = found
	}

	position := 0
	for _, env := range current.Environments {
		if env.Position >= position {
			position = env.Position + 1
		}
	}
	env := domain.Environment{ID: u.ids(), Name: name, Color: draft.Color, Position: position}
	if err := u.store.SaveEnvironment(ctx, workspace, env); err != nil {
		return domain.EnvState{}, err
	}
	if err := u.store.SetActiveEnvironment(ctx, workspace, env.ID); err != nil {
		return domain.EnvState{}, err
	}

	// The copies are numbered from one up, in the order the base lists them — which is the base's own
	// order, by position — exactly as a seeded environment numbers its first variable. Asking the
	// store for "the next position" would answer from a state read before this environment existed
	// and hand every copy the same row number.
	//
	// A database error halfway through the copy leaves a half-filled environment on the next read
	// rather than a state that cannot exist. Filling it again is what the user would do about it, and
	// the copy's names cannot collide — the base's are unique within a scope and the new scope is
	// empty.
	scope := domain.EnvScope{Environment: env.ID}
	for i, v := range base.Vars {
		if err := u.store.SaveVariable(ctx, workspace, scope,
			copiedVariable(v, u.ids(), i+1)); err != nil {
			return domain.EnvState{}, err
		}
	}
	return u.snapshot(ctx, workspace)
}

// copiedVariable is one variable of a base, reborn in a new environment: the same key, the same
// kind and the same place in the list. A secret arrives with an empty value; everything else
// arrives as it was — a baseUrl that differs only in the host is worth copying, a token is not.
func copiedVariable(v domain.Variable, id string, position int) domain.Variable {
	copied := domain.Variable{
		ID:       id,
		Name:     v.Name,
		Kind:     v.Kind,
		Enabled:  v.Enabled,
		Position: position,
	}
	if v.Kind != domain.VariableSecret {
		copied.Value = v.Value
	}
	return copied
}

// Update applies a patch. The readonly flag is what the design calls "Prod": it is the Access
// switch, and it closes the environment to every write the user could make — the check itself lives
// in the writers (see writable), and this is the one call that may still change an environment.
func (u *UseCase) Update(
	ctx context.Context,
	id string,
	patch EnvironmentPatch,
) (domain.EnvState, error) {
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
		// Refused the way a new one is: an empty name and a name that is too long are two different
		// things to say, and a bare sentinel leaves the window nothing to say either of them with.
		name, err := validEnvironmentName(*patch.Name)
		if err != nil {
			return domain.EnvState{}, err
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
	if err := u.store.SaveVariable(ctx, workspace, domain.EnvScope{Environment: env.ID},
		baseURL); err != nil {
		return domain.EnvState{}, err
	}
	if err := u.store.SetActiveEnvironment(ctx, workspace, env.ID); err != nil {
		return domain.EnvState{}, err
	}
	return u.snapshot(ctx, workspace)
}

func findEnvironment(state domain.EnvState, id string) (domain.Environment, bool) {
	for _, env := range state.Environments {
		if env.ID == id {
			return env, true
		}
	}
	return domain.Environment{}, false
}

// validEnvironmentName is a name an environment may carry, trimmed, or the refusal that says why it
// may not. Creating one and renaming one ask the same question, so they answer it the same way.
func validEnvironmentName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", domain.Refuse(domain.CodeNameEmpty, domain.ErrNotAllowed, nil)
	}
	if len([]rune(name)) > maxNameLength {
		return "", domain.Refuse(domain.CodeNameTooLong, domain.ErrNotAllowed,
			domain.Args{"max": strconv.Itoa(maxNameLength)})
	}
	return name, nil
}
