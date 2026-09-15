package record

import "json-inspector/internal/domain"

// What crosses the boundary: an attempt handed in, and the news of one coming back.

// RequestFinished carries the record the attempt produced — the same shape history lists, so the
// window has one type to draw and one place to put it.
type RequestFinished struct {
	ID     string        `json:"id"`
	Record domain.Record `json:"record"`
}

// RequestFailed is an attempt that produced no record at all, which is a failure of this side
// rather than of the network: a request that never reached the server still has a record.
type RequestFailed struct {
	ID    string `json:"id"`
	Error string `json:"error"`
	// Failure is the same news in the window's terms — the code and the values its sentence needs —
	// when the app is the side that refused. An error from the network has none, and the window shows
	// the message beside it.
	Failure *domain.Failure `json:"failure,omitempty"`
}

// SendInput is one attempt. The request in it is ready to go — its variables are already filled in
// — and the masked copy beside it is what history keeps.
//
// The two representations exist because the draft is still the window's: it holds the tokens, so it
// is the one that can say what a secret was. When the draft moves here, so does this.
type SendInput struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers []domain.HeaderPair `json:"headers"`
	Body    string              `json:"body"`
	// BodyKind and the two below it are what the body was rendered FROM. They travel beside the text
	// because a masked copy of a form or a file cannot be made from the text: it has to be rendered
	// again out of what the request is made of.
	BodyKind domain.BodyKind  `json:"bodyKind,omitempty"`
	Form     []domain.FormRow `json:"form,omitempty"`
	BodyFile string           `json:"bodyFile,omitempty"`

	MaskedURL     string              `json:"maskedUrl"`
	MaskedHeaders []domain.HeaderPair `json:"maskedHeaders"`
	MaskedBody    string              `json:"maskedBody"`
	Cookies       []domain.CookieRow  `json:"cookies"`

	// Digest is a credential the request cannot carry and the engine has to: see draft.Prepared.
	Digest *domain.DigestCredentials `json:"-"`

	// Node is the request this came from, when it came from a collection, and Run is the scope the
	// run's own variables live in — a collection run's id, or the send itself for a request sent on
	// its own. Both are what the scripts around the attempt are found by; both are empty for a
	// request composed on the command line, which has nothing above it.
	Node string `json:"node,omitempty"`
	Run  string `json:"run,omitempty"`
}

// IngestInput is a request this app did not send: one the browser made, as the extension reported
// it, or one that never left the machine at all. Source says which, and an empty one means the
// browser — the extension is the only thing that sends one of these from the outside.
type IngestInput struct {
	Source          domain.RecordSource `json:"source"`
	Method          string              `json:"method"`
	URL             string              `json:"url"`
	Status          int                 `json:"status"`
	StatusText      string              `json:"statusText"`
	ContentType     string              `json:"contentType,omitempty"`
	RequestHeaders  []domain.HeaderPair `json:"requestHeaders"`
	ResponseHeaders []domain.HeaderPair `json:"responseHeaders"`
	RequestBody     string              `json:"requestBody"`
	ResponseBody    string              `json:"responseBody"`
	DurationMs      int64               `json:"durationMs"`
	// Everything below only exists for a record that came from a browser tab. `omitempty` is what
	// says so on the wire, so a caller with no tab names none of them instead of spelling out zeros.
	StartedAt  int64  `json:"startedAt,omitempty"`
	WaitMs     int64  `json:"waitMs,omitempty"`
	DownloadMs int64  `json:"downloadMs,omitempty"`
	TabID      int    `json:"tabId,omitempty"`
	TabTitle   string `json:"tabTitle,omitempty"`
	TabURL     string `json:"tabURL,omitempty"`
	FavIconURL string `json:"favIconUrl,omitempty"`
}
