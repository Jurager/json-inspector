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
}
