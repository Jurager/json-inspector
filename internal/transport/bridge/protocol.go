package bridge

const DefaultPort = 38761

type CapturedRequest struct {
	Type            string            `json:"type"`
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
	WaitMs          int64             `json:"waitMs,omitempty"`
	DownloadMs      int64             `json:"downloadMs,omitempty"`
}

// CaptureState is what the extension says about itself. Paused is the extension's own fact and not
// something the app can read off the other fields: a paused capture and a connected extension with
// no tab under it look the same — nothing recorded, nothing to count.
type CaptureState struct {
	Type      string `json:"type"`
	Recording bool   `json:"recording"`
	Paused    bool   `json:"paused"`
	Tabs      int    `json:"tabs"`
	Browser   string `json:"browser"`
}

type FocusRequest struct {
	Type string `json:"type"`
	Tab  int    `json:"tab"`
}
