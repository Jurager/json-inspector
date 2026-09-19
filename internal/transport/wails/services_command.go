package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/draft"
)

// CommandService is the half of the command boundary that is not the draft: rendering a recorded
// request as a command a person can carry away. The rendering itself is a pure function of
// internal/usecase/draft; what cannot be done there is done here, because a command resolves
// `{{tokens}}` with secrets left masked, and the values behind them never reach the window.
type CommandService struct {
	records recordSource
	vars    variableSource
}

// recordSource is the history as a rendered command needs it: the request that was sent, and its
// body, which is not part of the summary a list carries. Declared here for the same reason as
// variableSource — this service is the pair, and neither feature knows about it.
type recordSource interface {
	Record(ctx context.Context, id string) (domain.Record, error)
	Body(ctx context.Context, id string, side domain.BodySide) (string, error)
}

// variableSource is the environments feature as a rendering needs it: the same shape the draft
// asks for, because it is the same question — what a text's `{{tokens}}` come to, a secret left as
// its mask. Declared here, in the composition, because neither feature knows about the other.
type variableSource interface {
	SubstituteTexts(
		ctx context.Context,
		above []domain.Variable,
		texts []string,
		mask bool,
		envID string,
	) ([]string, error)
}

func NewCommandService(records recordSource, vars variableSource) *CommandService {
	return &CommandService{records: records, vars: vars}
}

// Export writes a recorded request out as a command for one tool.
func (s *CommandService) Export(
	ctx context.Context,
	id string,
	format draft.CommandFormat,
) (string, error) {
	rec, err := s.records.Record(ctx, id)
	if err != nil {
		return "", err
	}
	body, err := s.records.Body(ctx, id, domain.SideRequest)
	if err != nil {
		return "", err
	}

	seed := draft.Seed{
		Method:  rec.Method,
		URL:     rec.URL,
		Body:    body,
		Headers: rec.RequestHeaders,
	}
	if err := s.substitute(ctx, &seed); err != nil {
		return "", err
	}
	return draft.RenderCommand(format, seed), nil
}

// substitute is a guard the export does not expect to need: a record is stored with its secrets
// already masked — prepare.go stores the hidden half of the substitution — so no `{{token}}` is
// normally left to resolve. It runs anyway, because a value must never leave as itself. All texts
// go at once, so a value appearing in two of them stays the same value in both.
func (s *CommandService) substitute(ctx context.Context, seed *draft.Seed) error {
	texts := []string{seed.URL, seed.Body}
	for _, header := range seed.Headers {
		texts = append(texts, header.Name, header.Value)
	}

	// A recorded request was already sent, and what it holds is what it holds: there is no tree above
	// it to answer for a name, so the environment answers alone. Nor is there a draft to have pinned
	// one of its own — a record is history, not something open in a card — so nothing is passed.
	resolved, err := s.vars.SubstituteTexts(ctx, nil, texts, true, "")
	if err != nil {
		return err
	}

	seed.URL, seed.Body = resolved[0], resolved[1]
	for i := range seed.Headers {
		seed.Headers[i].Name = resolved[2+2*i]
		seed.Headers[i].Value = resolved[3+2*i]
	}
	return nil
}
