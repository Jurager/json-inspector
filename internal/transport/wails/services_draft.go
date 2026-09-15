package wails

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/collection"
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
	drafts      *draft.UseCase
	collections *collection.UseCase
	host        *Host
}

func NewDraftService(drafts *draft.UseCase, collections *collection.UseCase, host *Host) *DraftService {
	return &DraftService{drafts: drafts, collections: collections, host: host}
}

// answered completes what the draft could not work out for itself, and every answer to the window
// passes through it — which is what makes this wrapper the one place a card's state is finished
// rather than one of the several a card is drawn from. See completed for what is filled in.
func (s *DraftService) answered(ctx context.Context, state draft.State, err error) (draft.State, error) {
	if err != nil {
		return state, err
	}
	return completed(ctx, s.drafts, s.collections, state), nil
}

// completed fills in what a draft could not work out for itself: what the levels above it answer,
// and the rows and the token that come of it. The draft cannot walk a tree — the tree's requests go
// through the draft, so asking one to know the other is asking each to be built first — and this
// package is where both are known, which is the same reason the tree is here at all.
//
// It is one function because the state a card opens with and the state it has after a keystroke
// have to be the same answer: a card that says nothing is above it, and whose header list is missing
// the row it inherits, is a card that lies until somebody touches it.
func completed(ctx context.Context, drafts *draft.UseCase, collections *collection.UseCase, state draft.State) draft.State {
	// A draft in no tree — the command line's — has nothing above it, and the walk is skipped rather
	// than run on every keystroke to find that out.
	if state.Draft.ID != domain.DraftCommandLine {
		// A tree that cannot be reached is a tree that said nothing: the request is still usable, and
		// the rows it would have inherited are the only thing missing.
		if above, err := collections.AuthFor(ctx, state.Draft.ID); err == nil {
			state.Inherited = above
		}
	}
	// A draft that answers for itself has already been projected from its own answer; only the one
	// that inherits had nothing to be projected from until the walk above.
	if state.Draft.Auth.Type != domain.AuthInherit {
		return state
	}
	if projected, err := drafts.Project(ctx, state.Draft, state.Inherited); err == nil {
		state.Projected = projected
		state.Token = drafts.Held(state.Draft, state.Inherited)
	}
	return state
}

// AuthSchemes is every way a request can authorize itself, in the order the window draws them: what
// each scheme asks for, how each field is drawn, and which of them are secrets. The window renders
// the Auth popover from this and holds no list of its own, so a scheme added here appears there.
func (s *DraftService) AuthSchemes() []domain.Scheme { return domain.AuthSchemes() }

// Snapshot is a draft as it stands, which is what a window opens on.
func (s *DraftService) Snapshot(ctx context.Context, id domain.DraftID) (draft.State, error) {
	state, err := s.drafts.Snapshot(ctx, id)
	return s.answered(ctx, state, err)
}

func (s *DraftService) SetMethod(ctx context.Context, id domain.DraftID, method string) (draft.State, error) {
	state, err := s.drafts.SetMethod(ctx, id, method)
	return s.answered(ctx, state, err)
}

func (s *DraftService) SetAuth(ctx context.Context, id domain.DraftID, auth domain.Auth) (draft.State, error) {
	state, err := s.drafts.SetAuth(ctx, id, auth)
	return s.answered(ctx, state, err)
}

// SetBodyKind picks the format the Body popover composes in. It is a call of its own rather than a
// row patch because it is a property of the request and not of any one row.
func (s *DraftService) SetBodyKind(ctx context.Context, id domain.DraftID, kind domain.BodyKind) (draft.State, error) {
	state, err := s.drafts.SetBodyKind(ctx, id, kind)
	return s.answered(ctx, state, err)
}

// SetBodyFile is the path a Binary body will be read from. The dialog that produced it is
// PickBodyFile below, and it is the window that decides which row the answer belongs to.
func (s *DraftService) SetBodyFile(ctx context.Context, id domain.DraftID, path string) (draft.State, error) {
	state, err := s.drafts.SetBodyFile(ctx, id, path)
	return s.answered(ctx, state, err)
}

// SetText is a buffer flush: the URL and the body are the window's while they are being typed, and
// this is how they reach this side. The revision travels back with the answer so the window can
// tell a reply to the keystroke it just made from one to the keystroke before it.
func (s *DraftService) SetText(ctx context.Context, id domain.DraftID, in draft.TextInput) (draft.TextResult, error) {
	result, err := s.drafts.SetText(ctx, id, in)
	if err != nil {
		return result, err
	}
	result.State, err = s.answered(ctx, result.State, nil)
	return result, err
}

func (s *DraftService) AddRow(ctx context.Context, id domain.DraftID, kind domain.RowKind) (draft.State, error) {
	state, err := s.drafts.AddRow(ctx, id, kind)
	return s.answered(ctx, state, err)
}

// RemoveRow and PatchRow address a row by id and not by position: a click that lands after the list
// changed under it must not delete the row that took its place.
func (s *DraftService) RemoveRow(ctx context.Context, draftID domain.DraftID, kind domain.RowKind, id string) (draft.State, error) {
	state, err := s.drafts.RemoveRow(ctx, draftID, kind, id)
	return s.answered(ctx, state, err)
}

func (s *DraftService) PatchRow(ctx context.Context, draftID domain.DraftID, kind domain.RowKind, id string, patch draft.RowPatch) (draft.State, error) {
	state, err := s.drafts.PatchRow(ctx, draftID, kind, id, patch)
	return s.answered(ctx, state, err)
}

// PatchDerived and RemoveDerived are the same edits made to a row the authorization projected rather
// than to one a person wrote. Such a row has no id — it is not stored — so it is named by what it is:
// which list it is in, what it is called, and for an edit, what it now says.
func (s *DraftService) PatchDerived(ctx context.Context, draftID domain.DraftID, target domain.RowKind, name string, value string) (draft.State, error) {
	state, err := s.drafts.PatchDerived(ctx, draftID, target, name, value)
	return s.answered(ctx, state, err)
}

// ObtainAuth and ForgetAuth are the two buttons the design gives a scheme that fetches: go and ask
// for a token, or throw the one there is away. What went wrong is reported rather than swallowed —
// the user asked in so many words, and the answer is what they are waiting for.
func (s *DraftService) ObtainAuth(ctx context.Context, draftID domain.DraftID) (draft.State, error) {
	state, err := s.drafts.ObtainAuth(ctx, draftID)
	return s.answered(ctx, state, err)
}

func (s *DraftService) ForgetAuth(ctx context.Context, draftID domain.DraftID) (draft.State, error) {
	state, err := s.drafts.ForgetAuth(ctx, draftID)
	return s.answered(ctx, state, err)
}

func (s *DraftService) RemoveDerived(ctx context.Context, draftID domain.DraftID) (draft.State, error) {
	state, err := s.drafts.RemoveDerived(ctx, draftID)
	return s.answered(ctx, state, err)
}

// Replace hands a draft a whole request: "открыть в запросе" on a record, or a command pasted into
// the command line.
func (s *DraftService) Replace(ctx context.Context, id domain.DraftID, seed draft.Seed) (draft.State, error) {
	state, err := s.drafts.Replace(ctx, id, seed)
	return s.answered(ctx, state, err)
}

// bodyFileFilter is deliberately wide: a form field can be any file, and offering "*.json" would be
// telling the user what they are about to send. The name it is offered under is the window's, for the
// same reason the title is: it is a word.
func bodyFileFilter(kindName string) []application.FileFilter {
	return []application.FileFilter{{DisplayName: kindName, Pattern: "*"}}
}

// PickBodyFile asks the user for the file a body carries and answers with its path, or with nothing
// when the dialog was closed — closing a dialog is not a failure.
//
// Nothing is written into the draft here: the window patches the row the answer belongs to, the way
// it patches a key or a value, so a pick travels as an edit and this answers only what was picked.
func (s *DraftService) PickBodyFile(ctx context.Context, title string, kindName string) (string, error) {
	return s.host.OpenFile(title, bodyFileFilter(kindName)...)
}
