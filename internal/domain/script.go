package domain

import "strings"

// Scripts is the code that runs around a request: before it goes out, and after it comes back.
//
// On a node it is a pointer for the reason Auth is — nil means "not set here" and a value means
// "this is what this level says". It does not win by being closest, though: every level that has a
// script runs, outermost first, so a collection can count requests and a request can assert about its
// own response without either of them replacing the other.
type Scripts struct {
	Pre  string `json:"pre,omitempty"`
	Post string `json:"post,omitempty"`
}

// Empty is the difference between a level that has nothing to say and one that says nothing.
func (s *Scripts) Empty() bool {
	if s == nil {
		return true
	}
	return strings.TrimSpace(s.Pre) == "" && strings.TrimSpace(s.Post) == ""
}

// ScriptScope is when a script ran: before the request was sent, or after the answer came back.
type ScriptScope string

const (
	ScriptPre  ScriptScope = "pre"
	ScriptPost ScriptScope = "post"
)

// ScriptInput is one script about to run: the code, when it runs, and everything it can see. What a
// script is handed is a request, and — after the answer came back — the response to it, because those
// are what it counts, changes and asserts about.
type ScriptInput struct {
	Scope  ScriptScope
	Source string

	// Request is what a script may change: a pre-request script edits the request that is about to go
	// out, and what it leaves here is what is sent.
	Request *ScriptRequest
	// Response is nil before the request has been sent, which is what a pre-request script finds when
	// it looks at pm.response.
	Response *Response

	// Variables is where the script reads and writes. The engine keeps no storage of its own: a value
	// a script writes goes back through here, so whether it outlives the run is decided in one place.
	Variables VarStore
}

// ScriptRequest is a request as a script sees it: the address it goes to, the headers as they will be
// sent, the body as it is written. Not the draft: a script counts what actually goes out, and a draft
// still holds `{{tokens}}` where the values will be.
type ScriptRequest struct {
	Method  string       `json:"method"`
	URL     string       `json:"url"`
	Headers []HeaderPair `json:"headers"`
	Body    string       `json:"body"`
}

// VarStore is where a script's variables live, under the three names the scripts themselves use.
// Reading `variables` reads the run's own scope and, through it, the environment and the globals —
// the order a request is resolved in; reading either of the other two reads that one alone, which is
// what tells a script whether a name is set in the environment or only borrowed from a run.
type VarStore interface {
	Get(scope string, name string) (string, bool)
	Set(scope string, name string, value string) error
}

const (
	// VarsRun lives as long as the run it belongs to and disappears with it.
	VarsRun         = "variables"
	VarsEnvironment = "environment"
	VarsGlobals     = "globals"
)

// ScriptRun is one execution of one script: what ran, whether it went through, what it printed and
// what it asserted. A run that failed is still a run — the failure is what the tab has to show.
type ScriptRun struct {
	ID       string      `json:"id"`
	RecordID string      `json:"recordId,omitempty"`
	NodeID   string      `json:"nodeId,omitempty"`
	Scope    ScriptScope `json:"scope"`

	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	DurationUs int64  `json:"durationUs"`
	CreatedAt  int64  `json:"createdAt"`

	// SkipRequest is a pre-request script's answer that this request should not go out at all.
	SkipRequest bool `json:"skipRequest,omitempty"`

	Logs  []ScriptLog  `json:"logs"`
	Tests []TestResult `json:"tests"`
}

// ScriptLog is one line a script printed. The level is the call it came from, so a warning does not
// have to be spelled out in the text.
type ScriptLog struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// TestResult is one assertion a script made. The name is the script's own — `pm.test('...')` — and a
// failure carries the reason beside it, because "не прошло" without why is not a report.
type TestResult struct {
	Name       string `json:"name"`
	Passed     bool   `json:"passed"`
	Error      string `json:"error,omitempty"`
	DurationUs int64  `json:"durationUs"`
}
