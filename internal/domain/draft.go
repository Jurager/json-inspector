package domain

// DraftID names one of the drafts the window is editing. It is a type of its own so that the one id
// written by hand — the command line's — reaches the window as a named constant rather than as a
// string spelled a second time, where a typo would be a draft that cannot be found at runtime.
type DraftID string

// DraftCommandLine is the id of the draft the window's command line edits. A collection node's
// draft is keyed by the node it came from, so the two are never the same draft.
const DraftCommandLine DraftID = "command-line"

// Row is one editable line of a draft: a query parameter or a header. It carries an id because the
// window addresses an edit by row and not by position — a patch that arrives after the row above it
// was removed would otherwise land on the wrong one.
type Row struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

// RowKind names which list of a draft a row belongs to. The window edits every one of them
// through the same four calls, so it is an argument rather than a set of methods per list.
type RowKind string

const (
	RowParams  RowKind = "params"
	RowHeaders RowKind = "headers"
	RowCookies RowKind = "cookies"
	RowForm    RowKind = "form"
)

// Draft is the request being composed. It holds tokens — `{{name}}` — and never the values behind
// them: a secret's value lives on the Go side of the boundary, so what is stored and sent back to
// the window is the text the user typed and nothing else.
//
// URL and Body are owned by the window while they are being typed in, which is why they travel as
// buffers rather than as patches to individual characters.
type Draft struct {
	ID       DraftID     `json:"id"`
	Revision int64       `json:"revision"`
	Method   string      `json:"method"`
	URL      string      `json:"url"`
	Params   []Row       `json:"params"`
	Headers  []Row       `json:"headers"`
	Auth     Auth        `json:"auth"`
	Body     string      `json:"body"`
	BodyKind BodyKind    `json:"bodyKind"`
	Form     []FormRow   `json:"form"`
	BodyFile string      `json:"bodyFile,omitempty"`
	Cookies  []CookieRow `json:"cookies"`
	// EnvironmentID pins this one request to an environment of its own, apart from the window's:
	// empty means it still follows whatever the window is on. It names an environment rather than
	// carrying one, the way ActiveID does, so a request stays pinned to the right thing across a
	// rename and reads as "nothing here" once more should that environment be deleted.
	EnvironmentID string `json:"environmentId,omitempty"`
}

// NewDraft is the shape a draft starts in: a GET with nothing in it. What the app puts in a fresh
// draft beyond that is the scenario's business, not the type's.
func NewDraft() Draft {
	return Draft{
		Method:  "GET",
		Params:  []Row{},
		Headers: []Row{},
		Auth:    Auth{Type: AuthNone},
		Cookies: []CookieRow{},
		Form:    []FormRow{},
	}
}
