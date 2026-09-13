package environment

import (
	"context"
	"fmt"
	"strings"

	"json-inspector/internal/domain"
	"json-inspector/internal/vars"
)

// Variable reads one variable by name in the scope a script names — the active environment, or the
// globals. It is the read behind `pm.environment.get` and `pm.globals.get`.
//
// Unlike the snapshot, it answers with the value, secrets included: a script that signs a request
// needs the token, and this value goes into the sandbox and nowhere else — not into the window, not
// into a log of ours. A script that prints it prints it itself, which is its own business.
func (u *UseCase) Variable(ctx context.Context, scope domain.VarScope, name string) (string, bool, error) {
	state, err := u.store.EnvState(ctx)
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
		return "", false, fmt.Errorf("область %q: %w", scope, domain.ErrNotAllowed)
	}
}

// SetVariable writes a variable by name, adding it when nobody has that name yet. Adding is the
// point: a script that records a token for the next request of a run must not have to know whether
// that variable exists, and the sheet is where names are looked at, not here.
func (u *UseCase) SetVariable(ctx context.Context, scope domain.VarScope, name string, value string) error {
	name = strings.TrimSpace(name)
	// A name no `{{token}}` can carry is a row nobody can use: the script would be writing into a
	// place the request it is running around cannot read.
	if name == "" || !vars.ValidName(name) {
		return fmt.Errorf("имя переменной %q: %w", name, domain.ErrNotAllowed)
	}

	state, err := u.store.EnvState(ctx)
	if err != nil {
		return err
	}

	where := domain.EnvScope{}
	switch scope {
	case domain.ScopeEnvironment:
		if _, ok := findEnvironment(state, state.ActiveID); !ok {
			return fmt.Errorf("окружение не выбрано, писать некуда")
		}
		where.Environment = state.ActiveID
	case domain.ScopeGlobals:
	default:
		return fmt.Errorf("область %q: %w", scope, domain.ErrNotAllowed)
	}

	if existing, ok := existing(scopeVariables(state, where), name); ok {
		existing.Value = value
		return u.store.SaveVariable(ctx, where, existing)
	}

	position, err := nextVariablePosition(state, where)
	if err != nil {
		return err
	}
	return u.store.SaveVariable(ctx, where, domain.Variable{
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
