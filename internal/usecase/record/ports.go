package record

import (
	"context"

	"json-inspector/internal/domain"
)

// Store is the history this feature keeps. Bodies go in whole and come back by reference, so a
// record on its way out never costs a document.
type Store interface {
	SaveRecord(ctx context.Context, rec domain.Record) error
	Record(ctx context.Context, id string) (domain.Record, error)
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

// Screener is the code that belongs around one attempt: the scripts of the collection the request came
// from, run before it goes out and after the answer came back. This feature owns the attempt and knows
// nothing about scripts — it says when, and somebody else says what.
//
// The signature is the shape the scripting feature already has, which is why the composition root
// binds it without an adapter: the port is declared here, next to the moment it describes.
//
// The pass goes in as a pointer because the first half fills it in: what the pre-request scripts did
// is carried back here and handed to the second half, where the record they belong to exists.
type Screener interface {
	Before(ctx context.Context, pass *domain.ScriptPass) (bool, error)
	After(ctx context.Context, pass domain.ScriptPass)
}

// Masker is a request as everything that outlives the send sees it: the same request with a secret's
// value left as its mask. The caller prepares one already, and this is asked again only for a request
// a script changed — the mask cannot be made where the values are not.
//
// The prepared attempt travels beside the script's answer because a body made of form rows or of a
// path to a file cannot be rendered from the text the script was shown: it has to be rendered again
// out of what the request is made of.
type Masker interface {
	Mask(ctx context.Context, in SendInput, sent domain.ScriptRequest) (Masked, error)
}

// Masked is a request written down: its address, its headers and its body with their secrets left as
// masks.
type Masked struct {
	URL     string
	Headers []domain.HeaderPair
	Body    string
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
