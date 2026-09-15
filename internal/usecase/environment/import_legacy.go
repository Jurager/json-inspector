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
// A secret comes over as its name and its kind and nothing else: values lived in the OS keychain
// then, which held them on one platform only and is gone now. Secrets are rows in the database
// here — where they are kept is the one thing this side does not decide — so a secret is a warning
// rather than a failure: the variable is there, and its value is one to enter again.
func (u *UseCase) ImportLegacy(ctx context.Context, raw string) (ImportReport, error) {
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

	if err := u.writeLegacy(ctx, *state, &report); err != nil {
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

// writeLegacy restores the old localStorage payload. It lands in the default workspace and not in
// whichever one happens to be on screen: the import is a one-time repair of what this installation
// kept before environments moved into the database, and it must not follow the user around spaces.
func (u *UseCase) writeLegacy(ctx context.Context, state legacyState, report *ImportReport) error {
	const workspace = domain.WorkspacePersonalID

	for i, legacyEnv := range state.Environments {
		env := domain.Environment{
			ID:       firstNonEmpty(legacyEnv.ID, u.ids()),
			Name:     firstNonEmpty(legacyEnv.Name, "Imported"),
			Color:    legacyEnv.Color,
			Readonly: legacyEnv.Readonly,
			Position: i + 1,
		}
		if err := u.store.SaveEnvironment(ctx, workspace, env); err != nil {
			return err
		}
		report.Environments++

		for j, legacyVar := range legacyEnv.Vars {
			v := u.legacyVariable(legacyVar, j+1, report)
			if err := u.store.SaveVariable(ctx, workspace, domain.EnvScope{Environment: env.ID},
				v); err != nil {
				return err
			}
			report.Variables++
		}
	}

	for j, legacyVar := range state.Globals {
		v := u.legacyVariable(legacyVar, j+1, report)
		if err := u.store.SaveVariable(ctx, workspace, domain.EnvScope{}, v); err != nil {
			return err
		}
		report.Variables++
	}

	if state.ActiveID != "" {
		if err := u.store.SetActiveEnvironment(ctx, workspace, state.ActiveID); err != nil {
			return err
		}
	}
	return nil
}

// legacyVariable carries one variable over. A text variable brings its value, which was in the
// payload. A secret brings its name and its kind and no value: what it held lived in the keychain,
// on one platform only, and the report names it rather than carrying a stale copy.
func (u *UseCase) legacyVariable(
	legacy legacyVariable,
	position int,
	report *ImportReport,
) domain.Variable {
	kind := domain.VariableText
	if legacy.Kind == string(domain.VariableSecret) {
		kind = domain.VariableSecret
	}

	v := domain.Variable{
		ID:       firstNonEmpty(legacy.ID, u.ids()),
		Name:     legacy.Name,
		Kind:     kind,
		Enabled:  legacy.Enabled,
		Position: position,
	}
	if kind != domain.VariableSecret {
		v.Value = legacy.Value
		return v
	}

	report.Secrets++
	report.Warnings = append(report.Warnings,
		fmt.Sprintf("secret %q: its value lived in the keychain, enter it again", legacy.Name))
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
