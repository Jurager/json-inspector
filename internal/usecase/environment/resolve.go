package environment

import (
	"context"

	"json-inspector/internal/domain"
)

// SubstituteTexts fills texts in with their variables, in the order they were given: a request is
// resolved as a whole, so its URL, body and headers agree with each other. The values stay on this
// side of the boundary.
//
// mask is the difference between the request that goes out and everything that outlives it: the
// preview, an export, the record. A secret leaves those as its mask.
func (u *UseCase) SubstituteTexts(
	ctx context.Context,
	texts []string,
	mask bool,
) ([]string, error) {
	resolver, err := u.resolver(ctx, !mask)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(texts))
	for i, text := range texts {
		out[i] = interpolate(text, resolver, mask)
	}
	return out, nil
}

// Missing lists the tokens in a set of texts that resolve to nothing, which is what blocks sending.
// Names repeat across texts — the same variable in a URL and in a header is one thing missing.
func (u *UseCase) Missing(ctx context.Context, texts []string) ([]string, error) {
	resolver, err := u.resolver(ctx, false)
	if err != nil {
		return nil, err
	}

	out := []string{}
	seen := map[string]bool{}
	for _, text := range texts {
		for _, name := range missing(text, resolver) {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}

// resolver is the lookup the `{{}}` grammar calls: the active environment wins over the globals,
// which is the order the design names — запрос → окружение → глобальные. Only the send path asks
// for a secret's value; everything else gets its kind and whether a value exists.
func (u *UseCase) resolver(ctx context.Context, revealSecrets bool) (lookup, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	state, err := u.store.EnvState(ctx, workspace)
	if err != nil {
		return nil, err
	}

	active := map[string]domain.Variable{}
	for _, env := range state.Environments {
		if env.ID != state.ActiveID {
			continue
		}
		for _, v := range env.Vars {
			if v.Enabled && v.Name != "" {
				active[v.Name] = v
			}
		}
	}

	globals := map[string]domain.Variable{}
	for _, v := range state.Globals {
		if v.Enabled && v.Name != "" {
			globals[v.Name] = v
		}
	}

	return func(name string) (domain.Resolution, bool) {
		if v, ok := active[name]; ok {
			return resolution(v, "env", revealSecrets), true
		}
		if v, ok := globals[name]; ok {
			return resolution(v, "global", revealSecrets), true
		}
		return domain.Resolution{}, false
	}, nil
}

func resolution(v domain.Variable, source string, reveal bool) domain.Resolution {
	out := domain.Resolution{
		Source:   source,
		Kind:     v.Kind,
		HasValue: v.Value != "",
	}
	if reveal || v.Kind != domain.VariableSecret {
		out.Value = v.Value
	}
	return out
}

// The lookup is scoped so that a patch proves the variable belongs where the caller says it does.
func findVariable(state domain.EnvState, scope domain.EnvScope, id string) (domain.Variable, bool) {
	for _, v := range scopeVariables(state, scope) {
		if v.ID == id {
			return v, true
		}
	}
	return domain.Variable{}, false
}
