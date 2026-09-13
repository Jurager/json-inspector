package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/scripting"
)

// ScriptingService is the code of a collection as the window works with it: what a level runs of its
// own, what runs around a node, and what the scripts of a record did.
//
// It holds both features because they answer different halves of one question and neither may import
// the other — the tree knows where a level is, the scripts know what it runs.
type ScriptingService struct {
	scripts     *scripting.UseCase
	collections *collection.UseCase
}

func NewScriptingService(scripts *scripting.UseCase, collections *collection.UseCase) *ScriptingService {
	return &ScriptingService{scripts: scripts, collections: collections}
}

// Scripts is what one level has of its own. Nil is "not set here" — the levels above are what runs —
// and that is a different answer from a script that is simply empty.
func (s *ScriptingService) Scripts(ctx context.Context, id string) (*domain.Scripts, error) {
	return s.collections.Scripts(ctx, id)
}

// SaveScripts writes what a level has of its own, and nil puts it back to inheriting. The answer is
// what was written, so the editor draws the state the database is in rather than the one it hoped for.
func (s *ScriptingService) SaveScripts(ctx context.Context, id string, scripts *domain.Scripts) (*domain.Scripts, error) {
	if err := s.collections.SaveScripts(ctx, id, scripts); err != nil {
		return nil, err
	}
	return s.collections.Scripts(ctx, id)
}

// Chain is what runs around a node, outermost first. The editor needs it to say what a level would
// inherit: the code a person sees when their own script is empty is somebody else's.
func (s *ScriptingService) Chain(ctx context.Context, nodeID string) ([]scripting.Level, error) {
	return s.scripts.Chain(ctx, nodeID)
}

// Runs is what the scripts of one record did, in the order they ran. An empty answer is a request
// nothing runs around, which is most of them.
func (s *ScriptingService) Runs(ctx context.Context, recordID string) ([]domain.ScriptRun, error) {
	return s.scripts.Runs(ctx, recordID)
}
