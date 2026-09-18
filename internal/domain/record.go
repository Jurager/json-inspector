package domain

import "time"

// RecordSource is where a record came from: the request builder or the browser extension. The two
// are kept in one table because they are drawn by one list and compared in one place.
type RecordSource string

const (
	SourceManual  RecordSource = "manual"
	SourceBrowser RecordSource = "browser"
)

// CookieRow is one row of the request-side jar the "Cookies" tab edits. Domain, path, expiry and
// the flags are Set-Cookie attributes rather than parts of a request's own Cookie header; they are
// kept so a draft can be restored from a record.
type CookieRow struct {
	// ID addresses the row while it is in a draft. A record keeps it as it was saved, which costs
	// nothing and lets the same jar be handed back to the draft it came from.
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Expires  string `json:"expires,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
}

// BodySide names one end of a record.
type BodySide string

const (
	SideRequest  BodySide = "request"
	SideResponse BodySide = "response"
)

// InlineBodyLimit is how much of a body travels with the record itself. A viewer usually opens the
// body it just received, so the common case should not cost a second call — and a list of two
// hundred records should still never carry two hundred documents.
const InlineBodyLimit = 64 << 10

// BodyRef points at one side of a record: the text itself when it is small, otherwise its size and
// a promise that the viewer can fetch it with a call of its own.
type BodyRef struct {
	Inline    string `json:"inline,omitempty"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
}

// PruneOptions is how much history is kept: at most MaxCount rows, and none older than MaxAge when
// that is set. Both apply — the count has always been capped; this adds age on top of it.
type PruneOptions struct {
	MaxCount int
	MaxAge   time.Duration
}

// HistoryStats is what one workspace's history weighs: how many requests it holds and how many
// bytes of bodies it keeps for them. It is a reading rather than a rule — what the settings screen
// shows beside the button that throws it away, so nobody has to guess what "clear" costs.
//
// Bodies rather than the database file: the file is shared by every space, and a number that
// counted the whole of it would change under a user who only captures in one.
type HistoryStats struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
}

// RecordSummary is a record without the parts only the detail pane needs — what a list row draws.
// The list reads the most rows, so it gets the least data.
type RecordSummary struct {
	ID string `json:"id"`
	// WorkspaceID is the space the record was made in. The list is one workspace's, so the window
	// needs it to tell a record that belongs on the screen from one that arrived from the extension
	// just after a switch — reading a record back is by id, and an id alone cannot say that.
	WorkspaceID string       `json:"workspaceId,omitempty"`
	Source      RecordSource `json:"source"`
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	Status      int          `json:"status"`
	StatusText  string       `json:"statusText"`
	ContentType string       `json:"contentType,omitempty"`
	Error       string       `json:"error,omitempty"`
	DurationUs  int64        `json:"durationUs"`
	StartedAt   int64        `json:"startedAt"`
	TabID       int          `json:"tabId,omitempty"`
	TabTitle    string       `json:"tabTitle,omitempty"`
	TabURL      string       `json:"tabURL,omitempty"`
	FavIconURL  string       `json:"favIconUrl,omitempty"`
}

// Record is a summary plus everything the response pane shows. The summary is embedded rather than
// copied, so the two can never disagree about which request this is.
type Record struct {
	RecordSummary

	// Cancelled is the engine saying the user stopped this attempt. Nothing records such a request —
	// there is nothing to show for one — so it is false for everything history holds; it is here
	// because a record is the shape of an attempt, and that is one of the things an attempt can be.
	Cancelled bool `json:"cancelled,omitempty"`
	// The phases, in microseconds, absent when they did not happen: a captured request carries only
	// what the page could time, and a repeat to a server it is already talking to dials nothing.
	DNSUs           *int64       `json:"dnsUs,omitempty"`
	ConnectUs       *int64       `json:"connectUs,omitempty"`
	TLSUs           *int64       `json:"tlsUs,omitempty"`
	WaitUs          *int64       `json:"waitUs,omitempty"`
	DownloadUs      *int64       `json:"downloadUs,omitempty"`
	RequestBytes    int64        `json:"requestBytes,omitempty"`
	ResponseBytes   int64        `json:"responseBytes,omitempty"`
	RequestHeaders  []HeaderPair `json:"requestHeaders"`
	ResponseHeaders []HeaderPair `json:"responseHeaders"`
	RequestCookies  []CookieRow  `json:"requestCookies,omitempty"`
	RequestBody     *BodyRef     `json:"requestBody,omitempty"`
	ResponseBody    *BodyRef     `json:"responseBody,omitempty"`
	// Skipped is a request that never went out, because a pre-request script said so. There is no
	// answer to fold in and nothing for history to keep — the flag is how whoever asked learns that
	// the request was not sent rather than that it failed.
	Skipped bool `json:"skipped,omitempty"`
}
