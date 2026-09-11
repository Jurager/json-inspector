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
}

type CaptureState struct {
	Type      string `json:"type"`
	Recording bool   `json:"recording"`
	Tabs      int    `json:"tabs"`
	Browser   string `json:"browser"`
}

type FocusRequest struct {
	Type string `json:"type"`
	Tab  int    `json:"tab"`
}
