package wails

import (
	"context"
	"errors"

	"json-inspector/internal/domain"
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
}

func NewRecordsService(records *record.UseCase, drafts *draft.UseCase) *RecordsService {
	return &RecordsService{records: records, drafts: drafts}
}

// Send starts the request the window is composing and answers with its id at once. The draft is
// read here rather than handed in: sending it means resolving its `{{tokens}}`, and a secret's
// value is on this side of the boundary — handing the window a request to send would mean handing
// it the secrets in it.
//
// What comes of the attempt arrives as an event, which is what lets the spinner belong to an id the
// window can cancel.
func (s *RecordsService) Send(ctx context.Context) (string, error) {
	prepared, err := s.drafts.Prepared(ctx)
	if err != nil {
		return "", err
	}
	return s.records.Send(ctx, recordInput(prepared))
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
		MaskedURL:     prepared.MaskedURL,
		MaskedHeaders: prepared.MaskedHeaders,
		MaskedBody:    prepared.MaskedBody,
		Cookies:       prepared.Cookies,
	}
}

func (s *RecordsService) Cancel(ctx context.Context, id string) (bool, error) {
	return s.records.Cancel(id), nil
}

// List is what the panel draws: everything but the body text, so two hundred rows do not carry two
// hundred documents and selecting one costs nothing until its body is opened.
func (s *RecordsService) List(ctx context.Context, source domain.RecordSource, limit int) ([]domain.Record, error) {
	return s.records.List(ctx, source, limit)
}

// Body is the call a viewer makes for a body that did not travel with the record — either because
// it is large or because the record arrived in a list. An absent side is an empty string, not an
// error: "this request had no body" is an answer, not a failure.
func (s *RecordsService) Body(ctx context.Context, id string, side domain.BodySide) (string, error) {
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
func (s *RecordsService) ImportLegacy(ctx context.Context, raw string) (record.ImportReport, error) {
	return s.records.ImportLegacy(ctx, raw)
}
