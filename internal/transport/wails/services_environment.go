package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/dotenv"
	"json-inspector/internal/infra/keychain"
	"json-inspector/internal/usecase/environment"
)

// EnvironmentsService is the environments screen and the request preview: the state they draw, the
// edits they make, and the resolution a draft's `{{tokens}}` need until that moves to Go too.
type EnvironmentsService struct {
	environments *environment.UseCase
	secrets      environment.SecretSource
}

func NewEnvironmentsService(envs *environment.UseCase, secrets environment.SecretSource) *EnvironmentsService {
	return &EnvironmentsService{environments: envs, secrets: secrets}
}

// Snapshot is the whole screen. Secret values are not in it: a secret says it has one, and Reveal
// is what shows it.
func (s *EnvironmentsService) Snapshot(ctx context.Context) (domain.EnvState, error) {
	return s.environments.Snapshot(ctx)
}

func (s *EnvironmentsService) CreateEnvironment(ctx context.Context, name string) (domain.EnvState, error) {
	return s.environments.Create(ctx, name)
}

func (s *EnvironmentsService) UpdateEnvironment(ctx context.Context, id string, patch environment.Patch) (domain.EnvState, error) {
	return s.environments.Update(ctx, id, patch)
}

func (s *EnvironmentsService) DeleteEnvironment(ctx context.Context, id string) (domain.EnvState, error) {
	return s.environments.Delete(ctx, id)
}

func (s *EnvironmentsService) ActivateEnvironment(ctx context.Context, id string) (domain.EnvState, error) {
	return s.environments.Activate(ctx, id)
}

func (s *EnvironmentsService) AddVariable(ctx context.Context, scope domain.EnvScope, kind domain.VariableKind) (domain.EnvState, error) {
	return s.environments.AddVariable(ctx, scope, kind)
}

func (s *EnvironmentsService) UpdateVariable(ctx context.Context, scope domain.EnvScope, patch environment.VariablePatch) (domain.EnvState, error) {
	return s.environments.UpdateVariable(ctx, scope, patch)
}

func (s *EnvironmentsService) RemoveVariable(ctx context.Context, scope domain.EnvScope, id string) (domain.EnvState, error) {
	return s.environments.RemoveVariable(ctx, scope, id)
}

func (s *EnvironmentsService) ImportEntries(ctx context.Context, scope domain.EnvScope, entries []dotenv.Entry) (domain.EnvState, error) {
	return s.environments.ImportEntries(ctx, scope, entries)
}

// Reveal is the eye button: the only call that hands a secret's value to the window.
func (s *EnvironmentsService) Reveal(ctx context.Context, id string) (string, error) {
	return s.environments.Reveal(ctx, id)
}

// ResolvedVariable is what the token tooltip needs. It is a struct because a bound method may only
// return a value and an error, and because "not found" is a normal answer rather than a failure.
type ResolvedVariable struct {
	Found      bool              `json:"found"`
	Resolution domain.Resolution `json:"resolution"`
}

func (s *EnvironmentsService) Resolve(ctx context.Context, name string) (ResolvedVariable, error) {
	resolution, found, err := s.environments.Resolve(ctx, name)
	if err != nil {
		return ResolvedVariable{}, err
	}
	return ResolvedVariable{Found: found, Resolution: resolution}, nil
}

// Substitute fills a text in with its variables; mask is what keeps a secret out of anything that
// outlives the moment of sending.
func (s *EnvironmentsService) Substitute(ctx context.Context, text string, mask bool) (string, error) {
	return s.environments.Substitute(ctx, text, mask)
}

func (s *EnvironmentsService) Missing(ctx context.Context, text string) ([]string, error) {
	return s.environments.Missing(ctx, text)
}

// ImportLegacy moves what the old frontend kept in localStorage into the database, once. The
// payload is the raw string: parsing the old shape is this side's job, not the window's.
func (s *EnvironmentsService) ImportLegacy(ctx context.Context, raw string) (environment.ImportReport, error) {
	return s.environments.ImportLegacy(ctx, raw, s.secrets)
}

// The three below are the keychain API the window used before variables moved into the database.
// They stay bound while the window is still the old one — the switch is its own change, and a
// binding that disappears under a running frontend breaks it — and go together with the keychain
// package a release after the import has run.

func (s *EnvironmentsService) SecretSet(envID, name, value string) error {
	return keychain.Set(envID, name, value)
}

func (s *EnvironmentsService) SecretGet(envID, name string) (string, error) {
	return keychain.Get(envID, name)
}

func (s *EnvironmentsService) SecretDelete(envID, name string) error {
	return keychain.Delete(envID, name)
}
