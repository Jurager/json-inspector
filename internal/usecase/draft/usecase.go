// Package draft owns the request being composed: its model, the transforms between the text the
// window types and the parts a request is made of, and the preview the command line draws.
//
// It owns one transform more than the typing: a command pasted into the line, read back into the
// same Seed that "Open in Request" and a saved request produce. Reading and writing commands live
// here rather than in a package of their own because a command *is* a request in another notation —
// the thing this package is about — and because the window that pastes one is pasting it into a
// draft.
package draft

import (
	"context"
	"errors"
	"strings"
	"sync"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// draftKey names a draft. The workspace is part of the address because the command line's draft is
// the same fixed id in every workspace.
type draftKey struct {
	workspace string
	id        domain.DraftID
}

// UseCase keeps the drafts the window is editing, addressed by id: the command line's, and one for
// the collection node whose card is open. They live in memory because a keystroke is a call, and
// only the command line's is written down: a card's draft is a proposal until it is saved.
type UseCase struct {
	mu     sync.Mutex
	store  Store
	scope  Scope
	vars   VariableSource
	files  FileSource
	auth   AuthMaterializer
	ids    platform.IDGen
	drafts map[draftKey]domain.Draft
}

func NewUseCase(
	store Store,
	scope Scope,
	vars VariableSource,
	files FileSource,
	auth AuthMaterializer,
	ids platform.IDGen,
) *UseCase {
	return &UseCase{
		store:  store,
		scope:  scope,
		vars:   vars,
		files:  files,
		auth:   auth,
		ids:    ids,
		drafts: map[draftKey]domain.Draft{},
	}
}

// current reads a draft, from memory or from the store on the first look. A workspace the window
// has not been in yet has nothing in memory, and that is a miss rather than an error: the command
// line of a space nobody has composed in starts as the fresh one.
func (u *UseCase) current(ctx context.Context, key draftKey) (domain.Draft, error) {
	u.mu.Lock()
	cached, ok := u.drafts[key]
	u.mu.Unlock()
	if ok {
		return cached, nil
	}

	stored, err := u.store.Draft(ctx, key.workspace, key.id)
	switch {
	case errors.Is(err, domain.ErrNotFound) && key.id == domain.DraftCommandLine:
		stored = u.fresh()
	case err != nil:
		return domain.Draft{}, err
	}

	u.mu.Lock()
	u.drafts[key] = stored
	u.mu.Unlock()
	return stored, nil
}

// Load reads the draft the last run left behind, and gives a window that has none the one this app
// has always started with.
func (u *UseCase) Load(ctx context.Context) error {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return err
	}
	stored, err := u.store.Draft(ctx, workspace, domain.DraftCommandLine)
	if errors.Is(err, domain.ErrNotFound) {
		stored = u.fresh()
	} else if err != nil {
		return err
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	u.drafts[draftKey{workspace: workspace, id: domain.DraftCommandLine}] = stored
	return nil
}

// Open puts a draft in memory under its own id, replacing whatever that id held. It is how a card
// opens a saved request — the node's fields become a draft — and how a save starts over: a draft
// that has just been opened has nothing unsaved in it.
//
// A card is the only thing that opens a draft that is not the command line's, and one card is open
// at a time, so the previous one is dropped rather than kept: a draft nobody is editing is memory
// nobody will free.
func (u *UseCase) Open(ctx context.Context, d domain.Draft) (State, error) {
	if d.ID == "" {
		return State{}, domain.Refuse(domain.CodeDraftWithoutID, domain.ErrNotAllowed, nil)
	}
	d = u.withRowIDs(d)
	// The rows follow the address, exactly as they do when it is typed: a request that came from a
	// file has an address and maybe no rows of its own, and a card that opened it would otherwise
	// show an empty Query list beside a filled-in query string.
	d.Params = u.rowsFromURL(d.URL, d.Params)
	// The revision counts the edits made since the draft was opened, and the window reads "nothing
	// unsaved" out of it — a draft that has just been opened is the saved request, not an edit of it.
	d.Revision = 0

	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return State{}, err
	}

	u.mu.Lock()
	for key := range u.drafts {
		if key.workspace == workspace && key.id != domain.DraftCommandLine {
			delete(u.drafts, key)
		}
	}
	u.drafts[draftKey{workspace: workspace, id: d.ID}] = d
	u.mu.Unlock()

	return u.stateOf(ctx, d)
}

// Current is the draft itself, without the preview: what the side that turns it back into a saved
// request needs. Reading costs nothing — no revision moves and nothing is written.
func (u *UseCase) Current(ctx context.Context, id domain.DraftID) (domain.Draft, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.Draft{}, err
	}
	return u.current(ctx, draftKey{workspace: workspace, id: id})
}

// Snapshot is the draft as it stands: what the window opens on, preview and all. Reading costs
// nothing — no revision moves and nothing is written, which is what a read has to mean.
func (u *UseCase) Snapshot(ctx context.Context, id domain.DraftID) (State, error) {
	draft, err := u.Current(ctx, id)
	if err != nil {
		return State{}, err
	}
	return u.stateOf(ctx, draft)
}

func (u *UseCase) SetMethod(ctx context.Context, id domain.DraftID, method string) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		d.Method = strings.TrimSpace(method)
		return nil
	})
}

// SetBodyKind changes the format the body is composed in. It is the same text seen three ways for
// JSON, XML and Raw, so nothing is cleared here: switching between them must not destroy what the
// window was holding, and neither must switching away to Form and back, which is why a form body
// and a file live in fields of their own.
func (u *UseCase) SetBodyKind(
	ctx context.Context,
	id domain.DraftID,
	kind domain.BodyKind,
) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		d.BodyKind = domain.KindOf(kind)
		return nil
	})
}

// SetBodyFile is the path the Binary body is read from at the moment of sending. It is a path and
// not the bytes: a draft holding a file would be holding a copy of something that may have changed
// since it was picked.
func (u *UseCase) SetBodyFile(ctx context.Context, id domain.DraftID, path string) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		d.BodyFile = strings.TrimSpace(path)
		return nil
	})
}

// SetText takes a buffer the window was typing into. The window keeps ownership of the text while
// it works — an answer never overwrites what is under the caret — so this is the only way a text
// the user typed reaches this side at all.
func (u *UseCase) SetText(
	ctx context.Context,
	id domain.DraftID,
	in TextInput,
) (TextResult, error) {
	result, err := u.result(ctx, id, func(d *domain.Draft) error {
		switch in.Field {
		case FieldURL:
			d.URL = in.Text
			// The rows follow the text: a parameter the URL no longer mentions is gone, and one it
			// gained is a row. Ids of the rows that stayed are kept, so an open editor keeps its
			// target.
			d.Params = u.rowsFromURL(in.Text, d.Params)
		case FieldBody:
			d.Body = in.Text
		default:
			return domain.Refuse(domain.CodeUnknownField, domain.ErrNotAllowed,
				domain.Args{"field": string(in.Field)})
		}
		return nil
	})
	if err != nil {
		return TextResult{}, err
	}
	return TextResult{Field: in.Field, Rev: in.Rev, State: result}, nil
}

// change applies an edit and keeps it. Every mutation goes through here, so the revision, what is
// returned and what is written down cannot disagree — and an edit is applied to a copy, so one that
// fails halfway leaves the draft exactly as it was.
//
// Only the command line's draft is written down: it is what the window opens on, while a card's
// draft becomes a saved request through its own gesture and nothing else.
func (u *UseCase) change(
	ctx context.Context,
	id domain.DraftID,
	edit func(*domain.Draft) error,
) (domain.Draft, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.Draft{}, err
	}
	key := draftKey{workspace: workspace, id: id}

	current, err := u.current(ctx, key)
	if err != nil {
		return domain.Draft{}, err
	}

	edited := copyOf(current)
	if err := edit(&edited); err != nil {
		return domain.Draft{}, err
	}
	edited.Revision++

	if id == domain.DraftCommandLine {
		if err := u.store.SaveDraft(ctx, workspace, edited); err != nil {
			return domain.Draft{}, err
		}
	}

	u.mu.Lock()
	u.drafts[key] = edited
	u.mu.Unlock()
	return edited, nil
}

func (u *UseCase) result(
	ctx context.Context,
	id domain.DraftID,
	edit func(*domain.Draft) error,
) (State, error) {
	draft, err := u.change(ctx, id, edit)
	if err != nil {
		return State{}, err
	}
	return u.stateOf(ctx, draft)
}

// copyOf is a draft whose row lists are its own. The struct copy alone shares those arrays with the
// draft in memory, and an edit applied through them would outlive the save that refused it — which
// is the one thing change promises cannot happen. The lists stay non-nil so that an empty one is
// still an empty list where the draft is written out.
func copyOf(d domain.Draft) domain.Draft {
	d.Params = append(make([]domain.Row, 0, len(d.Params)), d.Params...)
	d.Headers = append(make([]domain.Row, 0, len(d.Headers)), d.Headers...)
	d.Cookies = append(make([]domain.CookieRow, 0, len(d.Cookies)), d.Cookies...)
	d.Form = append(make([]domain.FormRow, 0, len(d.Form)), d.Form...)
	return d
}

func (u *UseCase) stateOf(ctx context.Context, draft domain.Draft) (State, error) {
	preview, err := u.preview(ctx, draft)
	if err != nil {
		return State{}, err
	}
	projected, err := u.projected(ctx, draft, nil)
	if err != nil {
		return State{}, err
	}
	return State{
		Draft:     draft,
		Preview:   preview,
		Projected: projected,
		Token:     u.Held(draft, nil),
	}, nil
}

// fresh is what a window with no stored draft starts on: a GET with nothing written in it. It used
// to seed a row — `Accept: application/vnd.api+json`, the type a JSON:API service wants — and that
// turned out to be a guess about the next request rather than a part of it: anyone sending
// elsewhere deleted the row first, every time. A header nobody asked for is the window writing the
// request for the user, and it is wrong as often as it is right.
func (u *UseCase) fresh() domain.Draft {
	draft := domain.NewDraft()
	draft.ID = domain.DraftCommandLine
	return draft
}
