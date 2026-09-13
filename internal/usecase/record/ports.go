package record

import (
	"context"

	"json-inspector/internal/domain"
)

// Store is the history this feature keeps. Bodies go in whole and come back by reference, so a
// record on its way out never costs a document.
type Store interface {
	SaveRecord(ctx context.Context, rec domain.Record) error
	Records(ctx context.Context, source domain.RecordSource, limit int) ([]domain.Record, error)
	ReadBody(ctx context.Context, id string, side domain.BodySide) (string, error)
	DeleteRecords(ctx context.Context, ids []string) error
	Prune(ctx context.Context, opts domain.PruneOptions) (int, error)

	ClaimImport(ctx context.Context, source string) (bool, error)
	FinishImport(ctx context.Context, source, status, detail string) error
}

// Request is a request on its way out. It is declared here rather than borrowed from the HTTP
// engine, so this feature does not depend on how a request leaves the process.
type Request struct {
	// ID names the attempt; the same id is what cancels it.
	ID      string
	Method  string
	URL     string
	Headers []domain.HeaderPair
	Body    string
}

// Executor sends a request. One implementation is the HTTP engine; a test can hand in its own.
type Executor interface {
	Execute(ctx context.Context, req Request) (*domain.Response, error)
	Cancel(id string) bool
}

// Notifier publishes what happened to whoever is listening.
type Notifier interface {
	Publish(topic string, payload any)
}

// RetentionSource answers how long history is kept — one question, so this feature does not depend
// on the settings screen. The composition root adapts the settings use case to it.
type RetentionSource interface {
	Retention(ctx context.Context) (domain.Retention, error)
}

// RetentionSourceFunc adapts a function to the port, which is what a test or the composition root
// usually has.
type RetentionSourceFunc func(ctx context.Context) (domain.Retention, error)

func (f RetentionSourceFunc) Retention(ctx context.Context) (domain.Retention, error) {
	return f(ctx)
}
