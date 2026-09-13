package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"json-inspector/internal/domain"
)

// LegacySource is the key the old frontend wrote its environments under. The import runs off the
// raw string for exactly this key, once, and the claim in data_imports is what makes it once.
const LegacySource = "localStorage:ji-env-v1"

// legacyState is the shape the TypeScript store persisted: the same fields, spelled the same way.
type legacyState struct {
	Environments []legacyEnvironment `json:"environments"`
	Globals      []legacyVariable    `json:"globals"`
	ActiveID     string              `json:"activeId"`
}

type legacyEnvironment struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Color    string           `json:"color"`
	Readonly bool             `json:"readonly"`
	Vars     []legacyVariable `json:"vars"`
}

type legacyVariable struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
}

// ImportReport says what an import did, so the screen can mention it once and the log keeps it.
type ImportReport struct {
	Environments int      `json:"environments"`
	Variables    int      `json:"variables"`
	Secrets      int      `json:"secrets"`
	Warnings     []string `json:"warnings,omitempty"`
	// Completed is false when the import had already run, or has nothing to read.
	Completed bool `json:"completed"`
}

// ImportLegacy moves the environments the old frontend kept in localStorage into the database.
//
// Secret values came from the OS keychain then and live in the database now: they are read once,
// here, through the SecretSource — which is why the keychain code outlives this import by a release
// and not longer. A secret that cannot be read is a warning, not a failure: the variable keeps its
// kind and an empty value, and the report names it.
func (u *UseCase) ImportLegacy(ctx context.Context, raw string, secrets SecretSource) (ImportReport, error) {
	var report ImportReport

	claimed, err := u.store.ClaimImport(ctx, LegacySource)
	if err != nil {
		return report, err
	}
	if !claimed {
		// Already done, or being done right now by another window.
		return report, nil
	}

	state, err := parseLegacy(raw)
	if err != nil {
		// A payload we cannot read is still a payload we are done with: retrying it forever would
		// keep the screen in "importing" for no reason.
		finish := u.store.FinishImport(ctx, LegacySource, "failed", err.Error())
		if finish != nil {
			log.Printf("[import] recording the failure: %v", finish)
		}
		return report, fmt.Errorf("reading ji-env-v1: %w", err)
	}
	if state == nil {
		if err := u.store.FinishImport(ctx, LegacySource, "done", "nothing to import"); err != nil {
			return report, err
		}
		return report, nil
	}

	if err := u.writeLegacy(ctx, *state, secrets, &report); err != nil {
		finish := u.store.FinishImport(ctx, LegacySource, "failed", err.Error())
		if finish != nil {
			log.Printf("[import] recording the failure: %v", finish)
		}
		return report, err
	}

	detail, err := json.Marshal(report)
	if err != nil {
		detail = []byte("{}")
	}
	if err := u.store.FinishImport(ctx, LegacySource, "done", string(detail)); err != nil {
		return report, err
	}
	report.Completed = true
	return report, nil
}

// parseLegacy reads the old payload. An empty or absent value returns a nil state — nothing to do —
// while a payload that exists but cannot be read is an error the caller records.
func parseLegacy(raw string) (*legacyState, error) {
	if raw == "" {
		return nil, nil
	}
	var state legacyState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, err
	}
	if len(state.Environments) == 0 && len(state.Globals) == 0 {
		return nil, nil
	}
	return &state, nil
}

func (u *UseCase) writeLegacy(ctx context.Context, state legacyState, secrets SecretSource, report *ImportReport) error {
	for i, legacyEnv := range state.Environments {
		env := domain.Environment{
			ID:       firstNonEmpty(legacyEnv.ID, u.ids()),
			Name:     firstNonEmpty(legacyEnv.Name, "Imported"),
			Color:    legacyEnv.Color,
			Readonly: legacyEnv.Readonly,
			Position: i + 1,
		}
		if err := u.store.SaveEnvironment(ctx, env); err != nil {
			return err
		}
		report.Environments++

		for j, legacyVar := range legacyEnv.Vars {
			v, err := u.legacyVariable(ctx, legacyVar, domain.EnvScope{Environment: env.ID}, j+1, secrets, report)
			if err != nil {
				return err
			}
			if err := u.store.SaveVariable(ctx, domain.EnvScope{Environment: env.ID}, v); err != nil {
				return err
			}
			report.Variables++
		}
	}

	for j, legacyVar := range state.Globals {
		v, err := u.legacyVariable(ctx, legacyVar, domain.EnvScope{}, j+1, secrets, report)
		if err != nil {
			return err
		}
		if err := u.store.SaveVariable(ctx, domain.EnvScope{}, v); err != nil {
			return err
		}
		report.Variables++
	}

	if state.ActiveID != "" {
		if err := u.store.SetActiveEnvironment(ctx, state.ActiveID); err != nil {
			return err
		}
	}
	return nil
}

// legacyVariable carries one variable over, pulling a secret's value out of the keychain on the way.
// The old key was the scope's id or the literal "globals", and the old account name was
// `env:<scope>:<name>` — both are that code's spelling, kept here on purpose.
func (u *UseCase) legacyVariable(
	ctx context.Context,
	legacy legacyVariable,
	scope domain.EnvScope,
	position int,
	secrets SecretSource,
	report *ImportReport,
) (domain.Variable, error) {
	kind := domain.VariableText
	if legacy.Kind == string(domain.VariableSecret) {
		kind = domain.VariableSecret
	}

	v := domain.Variable{
		ID:       firstNonEmpty(legacy.ID, u.ids()),
		Name:     legacy.Name,
		Value:    legacy.Value,
		Kind:     kind,
		Enabled:  legacy.Enabled,
		Position: position,
	}
	if kind != domain.VariableSecret || secrets == nil {
		return v, nil
	}

	report.Secrets++
	scopeKey := scope.Environment
	if scopeKey == "" {
		scopeKey = "globals"
	}
	value, err := secrets.Get(ctx, scopeKey, legacy.Name)
	switch {
	case err != nil:
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("secret %q was not read from the keychain: %v", legacy.Name, err))
	case value == "":
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("secret %q: no value in the keychain, enter it again", legacy.Name))
	default:
		v.Value = value
	}
	return v, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
