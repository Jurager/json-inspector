package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/dotenv"
	"json-inspector/internal/usecase/environment"
)

// EnvironmentsService is the environments screen: the state it draws and the edits it makes. What
// a `{{token}}` resolves to is no longer the window's question — the draft asks that side of the
// boundary itself, values and all.
type EnvironmentsService struct {
	environments *environment.UseCase
}

func NewEnvironmentsService(envs *environment.UseCase) *EnvironmentsService {
	return &EnvironmentsService{environments: envs}
}

// Snapshot is the whole screen. Secret values are not in it: a secret says it has one, and Reveal
// is what shows it.
func (s *EnvironmentsService) Snapshot(ctx context.Context) (domain.EnvState, error) {
	return s.environments.Snapshot(ctx)
}

func (s *EnvironmentsService) CreateEnvironment(
	ctx context.Context,
	name string,
) (domain.EnvState, error) {
	return s.environments.Create(ctx, name)
}

func (s *EnvironmentsService) UpdateEnvironment(
	ctx context.Context,
	id string,
	patch environment.EnvironmentPatch,
) (domain.EnvState, error) {
	return s.environments.Update(ctx, id, patch)
}

func (s *EnvironmentsService) DeleteEnvironment(
	ctx context.Context,
	id string,
) (domain.EnvState, error) {
	return s.environments.Delete(ctx, id)
}

func (s *EnvironmentsService) ActivateEnvironment(
	ctx context.Context,
	id string,
) (domain.EnvState, error) {
	return s.environments.Activate(ctx, id)
}

func (s *EnvironmentsService) AddVariable(
	ctx context.Context,
	scope domain.EnvScope,
	draft environment.VariableDraft,
) (domain.EnvState, error) {
	return s.environments.AddVariable(ctx, scope, draft)
}

// EnsureDefaults seeds a fresh database with the environment the app has always started with.
func (s *EnvironmentsService) EnsureDefaults(ctx context.Context) (domain.EnvState, error) {
	return s.environments.EnsureDefaults(ctx)
}

// ParseDotenv reads a .env file for the import dialog's preview: what is in it, and which names
// look like secrets. The dialog decides what to keep; parsing it is not the window's job.
func (s *EnvironmentsService) ParseDotenv(text string) []dotenv.Entry {
	return dotenv.Parse(text)
}

func (s *EnvironmentsService) UpdateVariable(
	ctx context.Context,
	scope domain.EnvScope,
	patch environment.VariablePatch,
) (domain.EnvState, error) {
	return s.environments.UpdateVariable(ctx, scope, patch)
}

func (s *EnvironmentsService) RemoveVariable(
	ctx context.Context,
	scope domain.EnvScope,
	id string,
) (domain.EnvState, error) {
	return s.environments.RemoveVariable(ctx, scope, id)
}

func (s *EnvironmentsService) ImportEntries(
	ctx context.Context,
	scope domain.EnvScope,
	entries []dotenv.Entry,
) (domain.EnvState, error) {
	return s.environments.ImportEntries(ctx, scope, entries)
}

// Reveal is the eye button: the only call that hands a secret's value to the window.
func (s *EnvironmentsService) Reveal(ctx context.Context, id string) (string, error) {
	return s.environments.Reveal(ctx, id)
}

// ImportLegacy moves what the old frontend kept in localStorage into the database, once. The
// payload is the raw string: parsing the old shape is this side's job, not the window's. A secret
// arrives without its value, which lived in the keychain and is not read any more — the report says
// so, and the value is one the user enters again.
func (s *EnvironmentsService) ImportLegacy(
	ctx context.Context,
	raw string,
) (environment.ImportReport, error) {
	return s.environments.ImportLegacy(ctx, raw)
}
