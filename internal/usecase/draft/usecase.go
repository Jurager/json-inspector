// Package draft owns the request being composed: its model, the transforms between the text the
// window types and the parts a request is made of, and the preview the command line draws.
package draft

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// UseCase keeps the drafts the window is editing, addressed by id: the command line's, and one for
// the collection node whose card is open.
//
// They live in memory because a keystroke is a call, and only the command line's is written down:
// that draft is a document the window opens on, while a card's draft is a proposal — saving it into
// the collection is a separate gesture, and until it is made, the request belongs to the window.
type UseCase struct {
	mu     sync.Mutex
	store  Store
	vars   VariableSource
	ids    platform.IDGen
	drafts map[domain.DraftID]domain.Draft
}

func NewUseCase(store Store, vars VariableSource, ids platform.IDGen) *UseCase {
	return &UseCase{
		store:  store,
		vars:   vars,
		ids:    ids,
		drafts: map[domain.DraftID]domain.Draft{},
	}
}

// TextField names one of the two texts the window owns while they are being typed in. Everything
// else about the draft is edited a row at a time, by id.
type TextField string

const (
	FieldURL  TextField = "url"
	FieldBody TextField = "body"
)

// TextInput is a buffer flush. Rev is the window's own counter, and it comes back with the answer:
// a reply to a keystroke that has since been typed over is recognised by it and dropped.
type TextInput struct {
	Field TextField `json:"field"`
	Text  string    `json:"text"`
	Rev   int64     `json:"rev"`
}

// State is the draft as it stands, with the preview that follows from it. Every answer carries
// both, so the window never has to ask what changed — and never has to work out for itself whether
// what it holds can go out.
type State struct {
	Draft   domain.Draft `json:"draft"`
	Preview Preview      `json:"preview"`
}

// TextResult is a buffer's answer. It carries back which buffer it is and the revision the window
// sent with it: a reply to a keystroke that has since been typed over is recognised by those two
// and dropped. The text itself is not in the answer — it is the window's until the window says
// otherwise, and an answer that overwrote it would be a character lost under the caret.
type TextResult struct {
	Field TextField `json:"field"`
	Rev   int64     `json:"rev"`
	State
}

// RowPatch is an edit to one row. Each field is optional, because a patch says what changed and
// nothing else: a row sent whole would undo a keystroke that landed between the two.
type RowPatch struct {
	Name     *string `json:"name,omitempty"`
	Value    *string `json:"value,omitempty"`
	Enabled  *bool   `json:"enabled,omitempty"`
	Domain   *string `json:"domain,omitempty"`
	Expires  *string `json:"expires,omitempty"`
	Secure   *bool   `json:"secure,omitempty"`
	HTTPOnly *bool   `json:"httpOnly,omitempty"`
}

// Seed is a whole request handed to the draft: what "открыть в запросе" and a pasted command both
// produce. Headers are pairs because a request can carry the same name twice.
type Seed struct {
	Method string `json:"method"`
	URL    string `json:"url"`
	Body   string `json:"body"`
	// A seed may carry neither list: a followed link has headers and no jar, a pasted command may
	// have neither. `omitempty` is what says so on the wire.
	Headers []domain.HeaderPair `json:"headers,omitempty"`
	Cookies []domain.CookieRow  `json:"cookies,omitempty"`
}

// Load reads the draft the last run left behind, and gives a window that has none the one this app
// has always started with.
func (u *UseCase) Load(ctx context.Context) error {
	stored, err := u.store.Draft(ctx, domain.DraftCommandLine)
	if errors.Is(err, domain.ErrNotFound) {
		stored = u.fresh()
	} else if err != nil {
		return err
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	u.drafts[domain.DraftCommandLine] = stored
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
		return State{}, fmt.Errorf("черновик без id: %w", domain.ErrNotAllowed)
	}
	d = u.withRowIDs(d)
	// The revision counts the edits made since the draft was opened, and the window reads "nothing
	// unsaved" out of it — a draft that has just been opened is the saved request, not an edit of it.
	d.Revision = 0

	u.mu.Lock()
	for id := range u.drafts {
		if id != domain.DraftCommandLine {
			delete(u.drafts, id)
		}
	}
	u.drafts[d.ID] = d
	u.mu.Unlock()

	return u.stateOf(ctx, d)
}

// Current is the draft itself, without the preview: what the side that turns it back into a saved
// request needs. Reading costs nothing — no revision moves and nothing is written.
func (u *UseCase) Current(ctx context.Context, id domain.DraftID) (domain.Draft, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	draft, ok := u.drafts[id]
	if !ok {
		return domain.Draft{}, fmt.Errorf("черновик %s: %w", id, domain.ErrNotFound)
	}
	return draft, nil
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

func (u *UseCase) SetAuth(ctx context.Context, id domain.DraftID, auth domain.Auth) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		d.Auth = auth
		return nil
	})
}

// SetText takes a buffer the window was typing into. The window keeps ownership of the text while
// it works — an answer never overwrites what is under the caret — so this is the only way a text
// the user typed reaches this side at all.
func (u *UseCase) SetText(ctx context.Context, id domain.DraftID, in TextInput) (TextResult, error) {
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
			return fmt.Errorf("поле %q: %w", in.Field, domain.ErrNotAllowed)
		}
		return nil
	})
	if err != nil {
		return TextResult{}, err
	}
	return TextResult{Field: in.Field, Rev: in.Rev, State: result}, nil
}

// AddRow appends an empty row to one of the three lists. Cookies are not rows: the jar carries the
// attributes a Set-Cookie reply has, and none of them belong to a request.
func (u *UseCase) AddRow(ctx context.Context, id domain.DraftID, kind domain.RowKind) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		switch kind {
		case domain.RowParams:
			d.Params = append(d.Params, domain.Row{ID: u.ids(), Enabled: true})
		case domain.RowHeaders:
			d.Headers = append(d.Headers, domain.Row{ID: u.ids(), Enabled: true})
		case domain.RowCookies:
			d.Cookies = append(d.Cookies, domain.CookieRow{ID: u.ids(), Path: "/"})
		default:
			return unknownKind(kind)
		}
		return nil
	})
}

// RemoveRow drops a row by id. A row that is not there is not an error: a double click, or a patch
// that arrives after its row is gone, has asked for exactly what it got.
func (u *UseCase) RemoveRow(ctx context.Context, draftID domain.DraftID, kind domain.RowKind, id string) (State, error) {
	return u.result(ctx, draftID, func(d *domain.Draft) error {
		switch kind {
		case domain.RowParams:
			d.Params = without(d.Params, id)
			syncURL(d)
		case domain.RowHeaders:
			d.Headers = without(d.Headers, id)
		case domain.RowCookies:
			d.Cookies = withoutCookie(d.Cookies, id)
		default:
			return unknownKind(kind)
		}
		return nil
	})
}

// PatchRow changes the fields a patch names and leaves the rest. A parameter edit writes the list
// back into the URL, because the URL is what goes out and the rows are only how it is edited.
func (u *UseCase) PatchRow(ctx context.Context, draftID domain.DraftID, kind domain.RowKind, id string, patch RowPatch) (State, error) {
	return u.result(ctx, draftID, func(d *domain.Draft) error {
		switch kind {
		case domain.RowParams:
			at := findRow(d.Params, id)
			if at < 0 {
				return notFound(kind, id)
			}
			applyPatch(&d.Params[at], patch)
			syncURL(d)
		case domain.RowHeaders:
			at := findRow(d.Headers, id)
			if at < 0 {
				return notFound(kind, id)
			}
			applyPatch(&d.Headers[at], patch)
		case domain.RowCookies:
			at := findCookie(d.Cookies, id)
			if at < 0 {
				return notFound(kind, id)
			}
			applyCookiePatch(&d.Cookies[at], patch)
		default:
			return unknownKind(kind)
		}
		return nil
	})
}

// Replace hands the draft a whole request. Everything the draft held is dropped, including the
// parameters the old URL carried and the choice made in the Auth chip: a seed is a request, not an
// edit to one.
func (u *UseCase) Replace(ctx context.Context, id domain.DraftID, seed Seed) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		d.Method = seed.Method
		d.URL = seed.URL
		d.Body = seed.Body
		d.Auth = domain.Auth{Type: domain.AuthNone}
		d.Params = u.rowsFromURL(seed.URL, nil)
		d.Headers = u.rowsFromHeaders(seed.Headers)
		d.Cookies = u.rowsFromCookies(seed)
		return nil
	})
}

// Prepared is the draft ready to go out: its variables filled in, and beside it the copy that
// outlives the moment of sending — a preview, an export, the record. Only this side ever sees the
// first one.
type Prepared struct {
	Method        string
	URL           string
	Headers       []domain.HeaderPair
	Body          string
	MaskedURL     string
	MaskedHeaders []domain.HeaderPair
	MaskedBody    string
	Cookies       []domain.CookieRow
}

func (u *UseCase) Prepared(ctx context.Context, id domain.DraftID) (Prepared, error) {
	draft, err := u.Current(ctx, id)
	if err != nil {
		return Prepared{}, err
	}
	return u.prepare(ctx, draft)
}

// Prepare fills in a request that is not the one being composed — a followed link, a collection run
// — without disturbing any draft. Resolving and masking live here and not with the caller, so there
// is one answer to what a request looks like when it leaves.
func (u *UseCase) Prepare(ctx context.Context, seed Seed) (Prepared, error) {
	draft := domain.Draft{
		Method:  seed.Method,
		URL:     seed.URL,
		Body:    seed.Body,
		Headers: u.rowsFromHeaders(seed.Headers),
		Cookies: seed.Cookies,
	}
	// A request with no jar of its own — a followed link, a pasted command — carries its cookies in
	// the header, and that is where they are read from.
	if len(draft.Cookies) == 0 {
		draft.Cookies = cookiesFromHeaders(seed.Headers)
	}
	return u.prepare(ctx, draft)
}

// change applies an edit and keeps it. Every mutation goes through here, so the revision, what is
// returned and what is written down cannot disagree — and an edit is applied to a copy, so one that
// fails halfway leaves the draft exactly as it was.
//
// Only the command line's draft is written down: it is what the window opens on, while a card's
// draft becomes a saved request through its own gesture and nothing else.
func (u *UseCase) change(ctx context.Context, id domain.DraftID, edit func(*domain.Draft) error) (domain.Draft, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	current, ok := u.drafts[id]
	if !ok {
		return domain.Draft{}, fmt.Errorf("черновик %s: %w", id, domain.ErrNotFound)
	}

	edited := current
	if err := edit(&edited); err != nil {
		return domain.Draft{}, err
	}
	edited.Revision++

	if id == domain.DraftCommandLine {
		if err := u.store.SaveDraft(ctx, edited); err != nil {
			return domain.Draft{}, err
		}
	}
	u.drafts[id] = edited
	return edited, nil
}

// result turns an edit into an answer: the draft that came of it, and the preview the command line
// draws from it.
func (u *UseCase) result(ctx context.Context, id domain.DraftID, edit func(*domain.Draft) error) (State, error) {
	draft, err := u.change(ctx, id, edit)
	if err != nil {
		return State{}, err
	}
	return u.stateOf(ctx, draft)
}

// withRowIDs gives every row an id. A draft that came from somewhere else — a saved request — has
// rows without them, and the window addresses a row by id: one that has none could not be edited.
func (u *UseCase) withRowIDs(d domain.Draft) domain.Draft {
	for i := range d.Params {
		if d.Params[i].ID == "" {
			d.Params[i].ID = u.ids()
		}
	}
	for i := range d.Headers {
		if d.Headers[i].ID == "" {
			d.Headers[i].ID = u.ids()
		}
	}
	for i := range d.Cookies {
		if d.Cookies[i].ID == "" {
			d.Cookies[i].ID = u.ids()
		}
		if d.Cookies[i].Path == "" {
			d.Cookies[i].Path = "/"
		}
	}
	return d
}

// stateOf is the answer to "what does this draft look like now": the draft itself, and the preview
// that follows from it.
func (u *UseCase) stateOf(ctx context.Context, draft domain.Draft) (State, error) {
	preview, err := u.preview(ctx, draft)
	if err != nil {
		return State{}, err
	}
	return State{Draft: draft, Preview: preview}, nil
}

// fresh is what a window with no stored draft starts on: the Accept header every JSON:API request
// needs. It is a row like any other, which is why the scenario and not the type puts it there.
func (u *UseCase) fresh() domain.Draft {
	draft := domain.NewDraft()
	draft.ID = domain.DraftCommandLine
	draft.Headers = []domain.Row{{ID: u.ids(), Name: "Accept", Value: "application/vnd.api+json", Enabled: true}}
	return draft
}

// rowsFromURL reads the query string into rows and keeps what the previous set of rows contributes:
// the ids of the ones that stayed, and the ones switched off, which the URL does not mention.
func (u *UseCase) rowsFromURL(raw string, existing []domain.Row) []domain.Row {
	rows := reconcile(paramsFromURL(raw), existing)
	for i := range rows {
		if rows[i].ID == "" {
			rows[i].ID = u.ids()
		}
	}
	return rows
}

// rowsFromHeaders turns the headers of a seed into rows. A `Cookie` header is kept: the jar is
// derived from it, and it is the collect step — not this one — that decides which of the two goes
// out when a seed brought both.
func (u *UseCase) rowsFromHeaders(headers []domain.HeaderPair) []domain.Row {
	rows := []domain.Row{}
	for _, header := range headers {
		rows = append(rows, domain.Row{ID: u.ids(), Name: header.Name, Value: header.Value, Enabled: true})
	}
	return rows
}

func (u *UseCase) rowsFromCookies(seed Seed) []domain.CookieRow {
	rows := seed.Cookies
	if len(rows) == 0 {
		rows = cookiesFromHeaders(seed.Headers)
	}

	out := make([]domain.CookieRow, 0, len(rows))
	for _, row := range rows {
		row.ID = u.ids()
		if row.Path == "" {
			row.Path = "/"
		}
		out = append(out, row)
	}
	return out
}

// cookiesFromHeaders is the fallback for a request that has no jar of its own — a pasted command,
// whose cookies are only ever a `Cookie` header.
func cookiesFromHeaders(headers []domain.HeaderPair) []domain.CookieRow {
	for _, header := range headers {
		if strings.EqualFold(header.Name, "Cookie") {
			return cookiesFromHeader(header.Value)
		}
	}
	return nil
}

// syncURL writes the parameter rows back into the URL, which is what actually goes out.
func syncURL(d *domain.Draft) {
	base, _ := queryOf(d.URL)
	d.URL = joinURL(base, d.Params)
}

func applyPatch(row *domain.Row, patch RowPatch) {
	if patch.Name != nil {
		row.Name = *patch.Name
	}
	if patch.Value != nil {
		row.Value = *patch.Value
	}
	if patch.Enabled != nil {
		row.Enabled = *patch.Enabled
	}
}

func applyCookiePatch(row *domain.CookieRow, patch RowPatch) {
	if patch.Name != nil {
		row.Name = *patch.Name
	}
	if patch.Value != nil {
		row.Value = *patch.Value
	}
	if patch.Domain != nil {
		row.Domain = *patch.Domain
	}
	if patch.Expires != nil {
		row.Expires = *patch.Expires
	}
	if patch.Secure != nil {
		row.Secure = *patch.Secure
	}
	if patch.HTTPOnly != nil {
		row.HTTPOnly = *patch.HTTPOnly
	}
}

// without returns the rows without the one named — a new slice, so that a change that cannot be
// saved can still be rolled back to the one before it.
func without(rows []domain.Row, id string) []domain.Row {
	out := make([]domain.Row, 0, len(rows))
	for _, row := range rows {
		if row.ID != id {
			out = append(out, row)
		}
	}
	return out
}

func withoutCookie(rows []domain.CookieRow, id string) []domain.CookieRow {
	out := make([]domain.CookieRow, 0, len(rows))
	for _, row := range rows {
		if row.ID != id {
			out = append(out, row)
		}
	}
	return out
}

func findRow(rows []domain.Row, id string) int {
	for i, row := range rows {
		if row.ID == id {
			return i
		}
	}
	return -1
}

func findCookie(rows []domain.CookieRow, id string) int {
	for i, row := range rows {
		if row.ID == id {
			return i
		}
	}
	return -1
}

func unknownKind(kind domain.RowKind) error {
	return fmt.Errorf("список %q: %w", kind, domain.ErrNotAllowed)
}

func notFound(kind domain.RowKind, id string) error {
	return fmt.Errorf("строка %s списка %q: %w", id, kind, domain.ErrNotFound)
}
