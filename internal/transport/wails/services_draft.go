package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/draft"
)

// DraftService is the request being composed. The window draws it and asks for changes; what those
// changes mean — which rows a URL has, what a `{{token}}` resolves to, whether the request can go
// out — is decided here.
//
// Every call names the draft it is about: the command line's, or the node a card is editing. The
// window holds both at once — switching between them must not lose the other — so nothing here
// assumes there is one.
type DraftService struct {
	drafts *draft.UseCase
}

func NewDraftService(drafts *draft.UseCase) *DraftService {
	return &DraftService{drafts: drafts}
}

// Snapshot is a draft as it stands, which is what a window opens on.
func (s *DraftService) Snapshot(ctx context.Context, id domain.DraftID) (draft.State, error) {
	return s.drafts.Snapshot(ctx, id)
}

func (s *DraftService) SetMethod(ctx context.Context, id domain.DraftID, method string) (draft.State, error) {
	return s.drafts.SetMethod(ctx, id, method)
}

func (s *DraftService) SetAuth(ctx context.Context, id domain.DraftID, auth domain.Auth) (draft.State, error) {
	return s.drafts.SetAuth(ctx, id, auth)
}

// SetText is a buffer flush: the URL and the body are the window's while they are being typed, and
// this is how they reach this side. The revision travels back with the answer so the window can
// tell a reply to the keystroke it just made from one to the keystroke before it.
func (s *DraftService) SetText(ctx context.Context, id domain.DraftID, in draft.TextInput) (draft.TextResult, error) {
	return s.drafts.SetText(ctx, id, in)
}

func (s *DraftService) AddRow(ctx context.Context, id domain.DraftID, kind domain.RowKind) (draft.State, error) {
	return s.drafts.AddRow(ctx, id, kind)
}

// RemoveRow and PatchRow address a row by id and not by position: a click that lands after the list
// changed under it must not delete the row that took its place.
func (s *DraftService) RemoveRow(ctx context.Context, draftID domain.DraftID, kind domain.RowKind, id string) (draft.State, error) {
	return s.drafts.RemoveRow(ctx, draftID, kind, id)
}

func (s *DraftService) PatchRow(ctx context.Context, draftID domain.DraftID, kind domain.RowKind, id string, patch draft.RowPatch) (draft.State, error) {
	return s.drafts.PatchRow(ctx, draftID, kind, id, patch)
}

// Replace hands a draft a whole request: "открыть в запросе" on a record, or a command pasted into
// the command line.
func (s *DraftService) Replace(ctx context.Context, id domain.DraftID, seed draft.Seed) (draft.State, error) {
	return s.drafts.Replace(ctx, id, seed)
}
