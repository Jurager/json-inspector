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
