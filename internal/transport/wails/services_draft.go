package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/draft"
)

// DraftService is the request the window is composing. The window draws it and asks for changes;
// what those changes mean — which rows a URL has, what a `{{token}}` resolves to, whether the
// request can go out — is decided here.
type DraftService struct {
	drafts *draft.UseCase
}

func NewDraftService(drafts *draft.UseCase) *DraftService {
	return &DraftService{drafts: drafts}
}

// Snapshot is the draft as it stands, which is what the window opens on.
func (s *DraftService) Snapshot(ctx context.Context) (draft.State, error) {
	return s.drafts.Snapshot(ctx)
}

func (s *DraftService) SetMethod(ctx context.Context, method string) (draft.State, error) {
	return s.drafts.SetMethod(ctx, method)
}

func (s *DraftService) SetAuth(ctx context.Context, auth domain.Auth) (draft.State, error) {
	return s.drafts.SetAuth(ctx, auth)
}

// SetText is a buffer flush: the URL and the body are the window's while they are being typed, and
// this is how they reach this side. The revision travels back with the answer so the window can
// tell a reply to the keystroke it just made from one to the keystroke before it.
func (s *DraftService) SetText(ctx context.Context, in draft.TextInput) (draft.TextResult, error) {
	return s.drafts.SetText(ctx, in)
}

func (s *DraftService) AddRow(ctx context.Context, kind domain.RowKind) (draft.State, error) {
	return s.drafts.AddRow(ctx, kind)
}

// RemoveRow and PatchRow address a row by id and not by position: a click that lands after the list
// changed under it must not delete the row that took its place.
func (s *DraftService) RemoveRow(ctx context.Context, kind domain.RowKind, id string) (draft.State, error) {
	return s.drafts.RemoveRow(ctx, kind, id)
}

func (s *DraftService) PatchRow(ctx context.Context, kind domain.RowKind, id string, patch draft.RowPatch) (draft.State, error) {
	return s.drafts.PatchRow(ctx, kind, id, patch)
}

// Replace hands the draft a whole request: "открыть в запросе" on a record, or a command pasted
// into the command line.
func (s *DraftService) Replace(ctx context.Context, seed draft.Seed) (draft.State, error) {
	return s.drafts.Replace(ctx, seed)
}
