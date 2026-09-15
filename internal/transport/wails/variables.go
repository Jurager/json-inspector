package wails

import (
	"context"

	"json-inspector/internal/usecase/environment"
)

// environmentVariables is the environments feature seen as the draft's variable source. The two
// speak the same shape already, so this is only where the composition names the pair — and where
// the draft stops knowing that variables come from an environment list at all.
type environmentVariables struct {
	environments *environment.UseCase
}

var _ interface {
	Missing(ctx context.Context, texts []string) ([]string, error)
	SubstituteTexts(ctx context.Context, texts []string, mask bool) ([]string, error)
} = environmentVariables{}

func (v environmentVariables) Missing(ctx context.Context, texts []string) ([]string, error) {
	return v.environments.Missing(ctx, texts)
}

func (v environmentVariables) SubstituteTexts(
	ctx context.Context,
	texts []string,
	mask bool,
) ([]string, error) {
	return v.environments.SubstituteTexts(ctx, texts, mask)
}
