package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/draft"
	"json-inspector/internal/usecase/record"
)

// collectionSender sends a run's request the way the command line does: through the draft, which
// fills in the `{{tokens}}` and keeps a secret out of what is written down, and then through the
// history. It is why the run feature knows about neither.
type collectionSender struct {
	drafts  *draft.UseCase
	records *record.UseCase
}

var _ collection.Sender = collectionSender{}

func (s collectionSender) Send(
	ctx context.Context,
	req collection.RunRequest,
) (domain.Record, error) {
	prepared, err := s.drafts.Prepare(ctx, draft.Seed{
		Method:   req.Method,
		URL:      req.URL,
		Body:     req.Body,
		BodyKind: req.BodyKind,
		Form:     req.Form,
		BodyFile: req.BodyFile,
		Headers:  req.Headers,
		Cookies:  req.Cookies,
		Auth:     req.Auth,
	})
	if err != nil {
		return domain.Record{}, err
	}

	input := recordInput(prepared)
	// The scripts of everything above this request run around it, and they are found by where it came
	// from: the node, and the run whose own variables they share.
	input.Node, input.Run = req.NodeID, req.Run
	// The space the run resolved travels with each request rather than being read again here: a run
	// of fifty requests must not scatter them across two spaces.
	return s.records.SendAndWait(ctx, req.Workspace, input)
}
