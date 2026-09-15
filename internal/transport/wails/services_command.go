package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/draft"
)

// CommandService is the one half of the command boundary that does not belong to the draft:
// writing a recorded request back out as a command a person can carry away. The other half —
// reading a pasted command and handing it to the draft — is a draft call, because that is what it
// does.
//
// The rendering itself is a pure function of internal/usecase/draft. What cannot be done there is
// done here: a command resolves `{{tokens}}` with a secret left masked, and the values that would
// resolve it never reach the window.
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
	SubstituteTexts(ctx context.Context, texts []string, mask bool) ([]string, error)
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

// substitute is the guard the export does not expect to need. A record is written with its secrets
// already masked — prepare.go stores the hidden half of the substitution, not the raw one — so
// there is normally no `{{token}}` left in one to resolve. It runs all the same, because what it
// protects is the one rule this whole path exists for: a value never leaves as itself. A record
// that somehow still held a token leaves as the mask rather than as `{{token}}`.
//
// Every text the command is made of goes at once: resolving them together is what keeps a value
// that appears in two of them the same value in both.
func (s *CommandService) substitute(ctx context.Context, seed *draft.Seed) error {
	texts := []string{seed.URL, seed.Body}
	for _, header := range seed.Headers {
		texts = append(texts, header.Name, header.Value)
	}

	resolved, err := s.vars.SubstituteTexts(ctx, texts, true)
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
