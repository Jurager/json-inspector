package wails

import (
	"context"

	"json-inspector/internal/command"
	"json-inspector/internal/domain"
)

// CommandService is the paste-and-copy pair: a command line read as a request, and a recorded
// request written back as one.
//
// Both halves are pure functions of internal/command — the package is the port of what used to live
// in the window, kept honest by a corpus dumped from it. This service is where they are bound, and
// where the one thing they cannot do alone is done for them: an export resolves `{{tokens}}` and
// leaves a secret masked, and the values that would resolve it never reach the window.
type CommandService struct {
	records recordSource
	vars    variableSource
}

// recordSource is the history as an export needs it: the request that was sent, and its body, which
// is not part of the summary a list carries. Declared here for the same reason as variableSource —
// this service is the pair, and neither feature knows about it.
type recordSource interface {
	Record(ctx context.Context, id string) (domain.Record, error)
	Body(ctx context.Context, id string, side domain.BodySide) (string, error)
}

// variableSource is the environments feature as an export needs it: the same shape the draft asks
// for, because it is the same question — what a text's `{{tokens}}` come to, a secret left as its
// mask. Declared here, in the composition, because neither feature knows about the other.
type variableSource interface {
	SubstituteTexts(ctx context.Context, texts []string, mask bool) ([]string, error)
}

func NewCommandService(records recordSource, vars variableSource) *CommandService {
	return &CommandService{records: records, vars: vars}
}

// Parse reads a pasted command line. The answer is the parser's, whole: a text that is not a
// command comes back as `none` rather than as a failure, because a person pasting an address into
// the address field has done nothing wrong.
func (s *CommandService) Parse(_ context.Context, text string) (command.Result, error) {
	return command.Parse(text), nil
}

// Export writes a recorded request as a command for one tool. keepTokens is a snippet shared with
// the tokens intact rather than the values they stand for, which is what a person pasting it into a
// collection wants: the variables are resolved where the request is sent, not where it is copied.
func (s *CommandService) Export(
	ctx context.Context,
	id string,
	format command.Format,
	keepTokens bool,
) (string, error) {
	rec, err := s.records.Record(ctx, id)
	if err != nil {
		return "", err
	}
	body, err := s.records.Body(ctx, id, domain.SideRequest)
	if err != nil {
		return "", err
	}

	req := command.Request{
		Method:  rec.Method,
		URL:     rec.URL,
		Body:    body,
		Headers: headerList(rec.RequestHeaders),
	}
	if keepTokens {
		return command.Export(format, req, command.ExportOptions{KeepTokens: true}), nil
	}
	if err := s.substitute(ctx, &req); err != nil {
		return "", err
	}
	return command.Export(format, req, command.ExportOptions{}), nil
}

// substitute fills the request's tokens in, masking a secret. Every text the command is made of
// goes at once: resolving them together is what keeps a value that appears in two of them the same
// value in both, and it is the answer the draft gets when it prepares the same request.
func (s *CommandService) substitute(ctx context.Context, req *command.Request) error {
	texts := []string{req.URL, req.Body}
	for _, header := range req.Headers {
		texts = append(texts, header.Name, header.Value)
	}

	resolved, err := s.vars.SubstituteTexts(ctx, texts, true)
	if err != nil {
		return err
	}

	req.URL, req.Body = resolved[0], resolved[1]
	for i := range req.Headers {
		req.Headers[i].Name = resolved[2+2*i]
		req.Headers[i].Value = resolved[3+2*i]
	}
	return nil
}

func headerList(pairs []domain.HeaderPair) []command.Header {
	out := make([]command.Header, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, command.Header{Name: pair.Name, Value: pair.Value})
	}
	return out
}
