package domain

// BodyKind is the format a request body is composed in. It belongs to the request and not to the
// window: the command line and a saved card both carry one, and the kind is what decides the
// Content-Type the request goes out with.
type BodyKind string

const (
	// BodyRaw is text this app does not interpret. It is the zero answer: a draft, a node or a file
	// written before there were kinds holds text, and text is what raw means.
	BodyRaw    BodyKind = "raw"
	BodyJSON   BodyKind = "json"
	BodyXML    BodyKind = "xml"
	BodyForm   BodyKind = "form"
	BodyBinary BodyKind = "binary"
)

// KindOf is the kind a body actually goes out as, with the empty value read as raw.
//
// There is deliberately no kind for "no body": an empty text already says that, and it is what
// keeps the columns written before this type existed meaningful.
func KindOf(kind BodyKind) BodyKind {
	if kind == "" {
		return BodyRaw
	}
	return kind
}

// FormRow is one line of a form body. It is not a Row: a form line is text until the paperclip is
// clicked, and a line that became a file keeps its path in Src rather than in Value, so that
// switching it back to text does not lose the text that was under it.
type FormRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Src     string `json:"src,omitempty"`
	File    bool   `json:"file,omitempty"`
	Enabled bool   `json:"enabled"`
}
