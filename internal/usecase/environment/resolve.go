package environment

import (
	"context"

	"json-inspector/internal/domain"
)

// SubstituteTexts fills texts in with their variables, in the order they were given: a request is
// resolved as a whole, so its URL, body and headers agree with each other. The values stay on this
// side of the boundary.
//
// above is what the collections around the request answer, outermost first and nearest last: they
// stand over the environment, because a collection that names `baseUrl` means that value for
// everything inside it. A request in the command line has none, and an empty set changes nothing.
//
// mask is the difference between the request that goes out and everything that outlives it: the
// preview, an export, the record. A secret leaves those as its mask.
func (u *UseCase) SubstituteTexts(
	ctx context.Context,
	above []domain.Variable,
	texts []string,
	mask bool,
) ([]string, error) {
	resolver, err := u.resolver(ctx, !mask, above)
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
func (u *UseCase) Missing(
	ctx context.Context,
	above []domain.Variable,
	texts []string,
) ([]string, error) {
	resolver, err := u.resolver(ctx, false, above)
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

// resolver is the lookup the `{{}}` grammar calls: the nearest level that answers wins, which is
// the order the design names — Request → Collection → Environment → Globals. A request's own
// answers never reach here: they are the text the draft holds, and this is the rest of it. Only the
// send path asks for a secret's value; everything else gets its kind and whether a value exists.
func (u *UseCase) resolver(
	ctx context.Context,
	revealSecrets bool,
	above []domain.Variable,
) (lookup, error) {
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

	// The collections around the request, nearest answer last: the list arrives outermost first, so
	// writing it in order leaves the level closest to the request standing.
	collections := map[string]domain.Variable{}
	for _, v := range above {
		if v.Enabled && v.Name != "" {
			collections[v.Name] = v
		}
	}

	return func(name string) (domain.Resolution, bool) {
		if v, ok := collections[name]; ok {
			return resolution(v, "collection", revealSecrets), true
		}
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
