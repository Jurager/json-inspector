package environment

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/vars"
)

// Resolve answers one `{{name}}` for the editor's tooltip. A secret reports that it has a value
// without showing it: the tooltip says "скрыто", and Reveal is the deliberate way to see it.
func (u *UseCase) Resolve(ctx context.Context, name string) (domain.Resolution, bool, error) {
	resolver, err := u.resolver(ctx, false)
	if err != nil {
		return domain.Resolution{}, false, err
	}
	value, ok := resolver(name)
	return value, ok, nil
}

// Substitute fills a text in with its variables. mask is the difference between the request that
// goes out and everything that outlives it: the preview, an export, the record — a secret leaves
// those as the mask.
func (u *UseCase) Substitute(ctx context.Context, text string, mask bool) (string, error) {
	resolver, err := u.resolver(ctx, !mask)
	if err != nil {
		return "", err
	}
	if mask {
		return vars.SubstituteMasked(text, resolver), nil
	}
	return vars.Substitute(text, resolver), nil
}

// ResolveTexts fills several texts in at once, in the order they were given. A request needs its
// URL, every header name and value, and its body resolved together, and one call keeps that a
// single round trip — and keeps the values on this side of the boundary: the window asks for a
// resolved request, not for the secrets in it.
func (u *UseCase) ResolveTexts(ctx context.Context, texts []string, mask bool) ([]string, error) {
	resolver, err := u.resolver(ctx, !mask)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(texts))
	for i, text := range texts {
		if mask {
			out[i] = vars.SubstituteMasked(text, resolver)
			continue
		}
		out[i] = vars.Substitute(text, resolver)
	}
	return out, nil
}

// Missing lists the tokens in a text that resolve to nothing, which is what blocks sending.
func (u *UseCase) Missing(ctx context.Context, text string) ([]string, error) {
	resolver, err := u.resolver(ctx, false)
	if err != nil {
		return nil, err
	}
	return vars.Missing(text, resolver), nil
}

// resolver is the lookup the `{{}}` grammar calls. The active environment wins over the globals,
// which is the order the design names: запрос → окружение → глобальные.
//
// revealSecrets decides whether a secret's value travels with the answer. Only the send path asks
// for it; everything else gets the kind and the fact that a value exists.
func (u *UseCase) resolver(ctx context.Context, revealSecrets bool) (vars.Resolver, error) {
	state, err := u.store.EnvState(ctx)
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

// findVariable looks a variable up by id inside one scope, which is how a patch proves it belongs
// where the caller says it does.
func findVariable(state domain.EnvState, scope domain.EnvScope, id string) (domain.Variable, bool) {
	for _, v := range scopeVariables(state, scope) {
		if v.ID == id {
			return v, true
		}
	}
	return domain.Variable{}, false
}
