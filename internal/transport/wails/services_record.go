package wails

import (
	"context"
	"errors"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/draft"
	"json-inspector/internal/usecase/record"
)

// RecordsService is the history and the requests that fill it: sending one, reading what came back,
// and forgetting what the retention rules no longer keep.
//
// It holds the draft as well as the history, and this is the one place that does: a request is
// composed by one feature and sent by another, and the layer that knows both is the layer that puts
// them together.
type RecordsService struct {
	records *record.UseCase
	drafts  *draft.UseCase
	// The tree is here for one question: what a request inside a collection inherits. The draft
	// cannot be asked it — a feature that owns a request must not know about the tree it sits in —
	// and this layer is the one that knows both.
	collections *collection.UseCase
}

func NewRecordsService(
	records *record.UseCase,
	drafts *draft.UseCase,
	collections *collection.UseCase,
) *RecordsService {
	return &RecordsService{records: records, drafts: drafts, collections: collections}
}

// Send starts the request a draft holds and answers with its id at once. The draft is read here
// rather than handed in: sending it means resolving its `{{tokens}}`, and a secret's value is on
// this side of the boundary — handing the window a request to send would mean handing it the
// secrets in it.
//
// The id names which draft: the command line's, or the collection node whose card asked to send —
// and sending a card's request does not save it, which is a gesture of its own.
//
// What comes of the attempt arrives as an event, which is what lets the spinner belong to an id the
// window can cancel.
func (s *RecordsService) Send(ctx context.Context, draftID domain.DraftID) (string, error) {
	inherits, err := s.inherited(ctx, draftID)
	if err != nil {
		return "", err
	}
	prepared, err := s.drafts.Prepared(ctx, draftID, inherits)
	if err != nil {
		return "", err
	}

	input := recordInput(prepared)
	// The draft is where the scripts around this request are found: a node of a collection brings the
	// ones above it, and the command line's draft brings the code of its own. Sending is the gesture
	// the scripts belong to, whichever of the two it is.
	input.Node = string(draftID)
	return s.records.Send(ctx, input)
}

// inherited is what a draft takes from the tree above it. The command line's draft is in no tree,
// and a node that has been deleted is in none either: both are answered with nothing to inherit,
// which is a real answer rather than a failure.
func (s *RecordsService) inherited(ctx context.Context, id domain.DraftID) (*domain.Auth, error) {
	auth, err := s.collections.AuthFor(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	return auth, err
}

// SendSpec starts a request that is not the one being composed — following a link out of a
// response, which must not disturb the draft the user is typing in.
func (s *RecordsService) SendSpec(ctx context.Context, seed draft.Seed) (string, error) {
	prepared, err := s.drafts.Prepare(ctx, seed)
	if err != nil {
		return "", err
	}
	return s.records.Send(ctx, recordInput(prepared))
}

// recordInput is where the two features meet: what a draft calls a request ready to go, the record
// calls one attempt.
func recordInput(prepared draft.Prepared) record.SendInput {
	return record.SendInput{
		Method:        prepared.Method,
		URL:           prepared.URL,
		Headers:       prepared.Headers,
		Body:          prepared.Body,
		BodyKind:      prepared.BodyKind,
		Form:          prepared.Form,
		BodyFile:      prepared.BodyFile,
		MaskedURL:     prepared.MaskedURL,
		MaskedHeaders: prepared.MaskedHeaders,
		MaskedBody:    prepared.MaskedBody,
		Cookies:       prepared.Cookies,
		Digest:        prepared.Digest,
	}
}

func (s *RecordsService) Cancel(ctx context.Context, id string) (bool, error) {
	return s.records.Cancel(id), nil
}

// List is what the panel draws: everything but the body text, so two hundred rows do not carry two
// hundred documents and selecting one costs nothing until its body is opened.
func (s *RecordsService) List(
	ctx context.Context,
	source domain.RecordSource,
	limit int,
) ([]domain.Record, error) {
	return s.records.List(ctx, source, limit)
}

// Record is one record by id. A run's row names the record it produced, and this is how the window
// opens it — the same thing a click on a history row does, for a row that is not in history.
func (s *RecordsService) Record(ctx context.Context, id string) (domain.Record, error) {
	return s.records.Record(ctx, id)
}

// Body is the call a viewer makes for a body that did not travel with the record — either because
// it is large or because the record arrived in a list. An absent side is an empty string, not an
// error: "this request had no body" is an answer, not a failure.
func (s *RecordsService) Body(
	ctx context.Context,
	id string,
	side domain.BodySide,
) (string, error) {
	body, err := s.records.Body(ctx, id, side)
	if errors.Is(err, domain.ErrNotFound) {
		return "", nil
	}
	return body, err
}

// Ingest stores a record the app did not send. The extension has this already, through the bridge;
// it is bound so the window can keep the sample it shows off with.
func (s *RecordsService) Ingest(ctx context.Context, in record.IngestInput) (domain.Record, error) {
	return s.records.Ingest(ctx, in)
}

func (s *RecordsService) Clear(ctx context.Context, ids []string) error {
	return s.records.Clear(ctx, ids)
}

// Prune applies the retention rules now, which is what the settings screen does when the window is
// shortened.
func (s *RecordsService) Prune(ctx context.Context) (int, error) {
	return s.records.Prune(ctx)
}

// ImportLegacy moves the history the old frontend kept in localStorage into the database, once. The
// payload is the raw string: reading that shape is this side's job, not the window's.
func (s *RecordsService) ImportLegacy(
	ctx context.Context,
	raw string,
) (record.ImportReport, error) {
	return s.records.ImportLegacy(ctx, raw)
}
