package bridge

// DefaultPort is the loopback port the app listens on for the browser
// extension. Both the app and the extension use this value by default.
const DefaultPort = 38761

// CapturedRequest is a request captured by the browser extension and pushed
// over the WebSocket. It is the wire format used by the "browser" mode.
type CapturedRequest struct {
	Type            string            `json:"type"` // always "request"
	ID              string            `json:"id"`
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	RequestHeaders  map[string]string `json:"requestHeaders"`
	RequestBody     string            `json:"requestBody"`
	Status          int               `json:"status"`
	StatusText      string            `json:"statusText"`
	ResponseHeaders map[string]string `json:"responseHeaders"`
	ResponseBody    string            `json:"responseBody"`
	DurationMs      int64             `json:"durationMs"`
	StartedAt       int64             `json:"startedAt"`
	TabID           int               `json:"tabId"`
	TabTitle        string            `json:"tabTitle"`
	TabURL          string            `json:"tabURL"`
	FavIconURL      string            `json:"favIconUrl"`
}

// CaptureState is the second wire-format message, sent by the extension to
// report its live capture status (how many tabs, whether recording, which
// browser) so the status bar doesn't have to guess from badge counts.
type CaptureState struct {
	Type      string `json:"type"` // always "state"
	Recording bool   `json:"recording"`
	Tabs      int    `json:"tabs"`
	Browser   string `json:"browser"`
}
