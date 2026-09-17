package bridge

import "json-inspector/internal/domain"

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
	// TabList is what Tabs counts, one entry each: which tabs are under capture and since when. It is
	// a second field rather than a richer Tabs because a frame already on the wire is a promise: an
	// app that is one version behind reads the count and knows no better, and a window that knows the
	// times can say since when.
	TabList []TabState `json:"tabList,omitempty"`
}

// TabState is one tab under capture. Since is unix milliseconds, the moment the tab was armed — not
// the time of its first request, which is a different and much later thing.
type TabState struct {
	TabID int   `json:"tabId"`
	Since int64 `json:"since"`
}

// FocusRequest is the extension asking the window to look at a tab.
type FocusRequest struct {
	Type string `json:"type"`
	Tab  int    `json:"tab"`
}

// FiltersFrame is the app telling the extension what to keep. The rules are the app's own type and
// not a second shape of them: there is nothing to translate between what the settings screen stores
// and what the extension enforces, and a copy here would be a copy that drifts.
//
// Nothing is answered: this protocol has no replies, and the extension's next state frame is the
// only echo that a frame arrived.
type FiltersFrame struct {
	Type    string                `json:"type"`
	Filters domain.CaptureFilters `json:"filters"`
}
