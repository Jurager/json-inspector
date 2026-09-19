package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/infra/sqlite"
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
	// The pin travels with the seed, so a request saved against one environment is resolved against
	// it wherever it is sent from — the run is not looking at the tree, and the window may well be on
	// a different environment by the time the twentieth request goes out.
	prepared, err := s.drafts.Prepare(ctx, draft.Seed{
		Method:        req.Method,
		URL:           req.URL,
		Body:          req.Body,
		BodyKind:      req.BodyKind,
		Form:          req.Form,
		BodyFile:      req.BodyFile,
		Headers:       req.Headers,
		Cookies:       req.Cookies,
		Auth:          req.Auth,
		EnvironmentID: req.EnvironmentID,
	}, req.Variables)
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

// assertions answers a run's row with what the scripts around its request asserted. The reports are
// the scripting feature's rows, and reading them through the store is what keeps the two features
// from having to know each other — the same store that answers both is the composition layer's to
// bind.
type assertions struct{ store *sqlite.Store }

var _ collection.Assertions = assertions{}

func (a assertions) Assertions(ctx context.Context, recordID string) (int, int, error) {
	runs, err := a.store.ScriptRuns(ctx, recordID)
	if err != nil {
		return 0, 0, err
	}
	passed, total := 0, 0
	for _, run := range runs {
		for _, test := range run.Tests {
			total++
			if test.Passed {
				passed++
			}
		}
	}
	return passed, total, nil
}
