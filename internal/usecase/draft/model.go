package draft

import "json-inspector/internal/domain"

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
	// Projected is what the authorization puts in the parameter and header lists, which is not part
	// of the draft and is not stored with it: it is what the draft's auth comes to, worked out on
	// every answer so the two can never disagree.
	Projected []domain.ProjectedRow `json:"projected"`
	// Token is the state of a credential somebody else issues, for the schemes that have one. It is
	// absent for a scheme that carries what it was given: there is nothing to say about a token the
	// user typed, and a block saying so would be noise.
	Token *domain.AuthToken `json:"token,omitempty"`
	// Inherited is what the levels above this draft answer with, for a draft that is a node of a
	// tree: the nearest one that gave a credential, or nothing when none did. It is not a getter the
	// window could have written for itself — «None» on a folder is a level the walk goes past, and
	// that rule belongs with the walk.
	Inherited *domain.Auth `json:"inherited,omitempty"`
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
	// Src and File belong to a form row: the path a file field carries, and whether it is one.
	Src  *string `json:"src,omitempty"`
	File *bool   `json:"file,omitempty"`
}

// Seed is a whole request handed to the draft: what "Open in Request" and a pasted command both
// produce. Headers are pairs because a request can carry the same name twice.
type Seed struct {
	Method string `json:"method"`
	URL    string `json:"url"`
	Body   string `json:"body"`
	// A seed carries the format too: a followed link or a pasted command is a whole request, and the
	// Content-Type that follows from the kind is part of what it goes out with.
	BodyKind domain.BodyKind  `json:"bodyKind,omitempty"`
	Form     []domain.FormRow `json:"form,omitempty"`
	BodyFile string           `json:"bodyFile,omitempty"`
	// A seed may carry neither list: a followed link has headers and no jar, a pasted command may
	// have neither. `omitempty` is what says so on the wire.
	Headers []domain.HeaderPair `json:"headers,omitempty"`
	Cookies []domain.CookieRow  `json:"cookies,omitempty"`
	// Auth is what the request authorizes itself with. It travels resolved: whoever builds a seed
	// knows where the request came from, and a seed is a request, not a place in a tree.
	Auth *domain.Auth `json:"auth,omitempty"`
}

// Prepared is the draft ready to go out: its variables filled in, and beside it the copy that
// outlives the moment of sending — a preview, an export, the record. Only this side ever sees the
// first one.
type Prepared struct {
	Method  string
	URL     string
	Headers []domain.HeaderPair
	Body    string
	// The format and what the body was rendered from travel beside the text: a masked copy of a form
	// or of a file cannot be made from the text, so whoever masks this request again needs these.
	BodyKind domain.BodyKind
	Form     []domain.FormRow
	BodyFile string

	MaskedURL     string
	MaskedHeaders []domain.HeaderPair
	MaskedBody    string
	Cookies       []domain.CookieRow

	// Digest is a credential the request cannot carry: it goes to the engine, which is the only side
	// that can be there when the server says how. Nothing above the engine has a header to show for
	// it, which is why it travels beside them rather than among them.
	Digest *domain.DigestCredentials
}
