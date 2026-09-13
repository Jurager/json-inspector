package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/draft"
	"json-inspector/internal/usecase/record"
)

// collectionSender is a saved request of a run, sent the way the command line sends one: through
// the draft, which fills in the `{{tokens}}` and keeps a secret's value out of what is written
// down, and then through the history, which records it.
//
// This is the whole reason the run feature does not know about either: it hands over a request and
// gets back what came of it.
type collectionSender struct {
	drafts  *draft.UseCase
	records *record.UseCase
}

var _ collection.Sender = collectionSender{}

func (s collectionSender) Send(ctx context.Context, req collection.RunRequest) (domain.Record, error) {
	prepared, err := s.drafts.Prepare(ctx, draft.Seed{
		Method:  req.Method,
		URL:     req.URL,
		Body:    req.Body,
		Headers: req.Headers,
		Cookies: req.Cookies,
	})
	if err != nil {
		return domain.Record{}, err
	}

	input := recordInput(prepared)
	// The scripts of everything above this request run around it, and they are found by where it came
	// from: the node, and the run whose own variables they share.
	input.Node, input.Run = req.NodeID, req.Run
	return s.records.SendAndWait(ctx, input)
}
