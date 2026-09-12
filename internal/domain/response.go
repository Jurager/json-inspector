package domain

// Response is what a request comes back as: the body plus the per-phase timings.
type Response struct {
	Status      int               `json:"status"`
	StatusText  string            `json:"statusText"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	DurationMs  int64             `json:"durationMs"`
	ContentType string            `json:"contentType"`
	Error       string            `json:"error,omitempty"`
	Cancelled   bool              `json:"cancelled,omitempty"`
	DNSMs       int64             `json:"dnsMs,omitempty"`
	ConnectMs   int64             `json:"connectMs,omitempty"`
	TLSMs       int64             `json:"tlsMs,omitempty"`
	WaitMs      int64             `json:"waitMs,omitempty"`
	DownloadMs  int64             `json:"downloadMs,omitempty"`
}
