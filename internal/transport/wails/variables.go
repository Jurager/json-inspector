package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/environment"
)

// environmentVariables is the environments feature seen as the draft's variable source. The two
// already speak the same shape, so this is only where the composition names the pair — and where
// the draft stops knowing that variables come from an environment list at all. What the
// collections over a request answer travels with every call: the draft is handed it by whoever
// knows the tree, just as it is handed the authorization those levels answer with.
type environmentVariables struct {
	environments *environment.UseCase
}

var _ interface {
	Missing(
		ctx context.Context,
		above []domain.Variable,
		texts []string,
		envID string,
	) ([]string, error)
	SubstituteTexts(
		ctx context.Context,
		above []domain.Variable,
		texts []string,
		mask bool,
		envID string,
	) ([]string, error)
} = environmentVariables{}

func (v environmentVariables) Missing(
	ctx context.Context,
	above []domain.Variable,
	texts []string,
	envID string,
) ([]string, error) {
	return v.environments.Missing(ctx, above, texts, envID)
}

func (v environmentVariables) SubstituteTexts(
	ctx context.Context,
	above []domain.Variable,
	texts []string,
	mask bool,
	envID string,
) ([]string, error) {
	return v.environments.SubstituteTexts(ctx, above, texts, mask, envID)
}

// activeEnvironment is the environments feature seen as what a run goes out under. The name is read
// once, when the run starts, and kept with the run: the window may be on another environment by the
// time the page that reports on the run is read, and the run is not.
type activeEnvironment struct {
	environments *environment.UseCase
}

var _ collection.Environment = activeEnvironment{}

func (a activeEnvironment) ActiveEnvironment(ctx context.Context) (string, error) {
	state, err := a.environments.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	for _, one := range state.Environments {
		if one.ID == state.ActiveID {
			return one.Name, nil
		}
	}
	// Nothing chosen: the run went out on the globals alone, and an empty name is what says so.
	return "", nil
}
