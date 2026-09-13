package domain

import "sort"

// HeaderPair is one header line. Headers travel as a sequence rather than a map because the wire
// allows a name to repeat — two Set-Cookie lines are two cookies — and a map would keep one.
type HeaderPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PairsFromMap turns a header map into pairs. A map has no order of its own, so the pairs come out
// sorted: two runs over the same headers must not disagree, and neither must two ends of the app.
func PairsFromMap(headers map[string]string) []HeaderPair {
	out := make([]HeaderPair, 0, len(headers))
	for name, value := range headers {
		out = append(out, HeaderPair{Name: name, Value: value})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

// Response is what a request comes back as: the body plus the per-phase timings.
type Response struct {
	Status      int          `json:"status"`
	StatusText  string       `json:"statusText"`
	Headers     []HeaderPair `json:"headers"`
	Body        string       `json:"body"`
	DurationMs  int64        `json:"durationMs"`
	ContentType string       `json:"contentType"`
	Error       string       `json:"error,omitempty"`
	Cancelled   bool         `json:"cancelled,omitempty"`
	DNSMs       int64        `json:"dnsMs,omitempty"`
	ConnectMs   int64        `json:"connectMs,omitempty"`
	TLSMs       int64        `json:"tlsMs,omitempty"`
	WaitMs      int64        `json:"waitMs,omitempty"`
	DownloadMs  int64        `json:"downloadMs,omitempty"`
	// HasTiming separates "the phases were measured" from "they all read zero": a connection reused
	// from the pool spends no time on DNS, connect or TLS, and a request that never got an answer
	// has no phases at all. A viewer draws the first and says so about the second.
	HasTiming bool `json:"hasTiming,omitempty"`
	// BodyTruncated says the body was cut at the engine's cap, so a viewer can say so instead of
	// showing half a document as if it were all of it.
	BodyTruncated bool `json:"bodyTruncated,omitempty"`
}

// HeaderValues lists every value recorded for a name, in the order the server sent them.
func (r *Response) HeaderValues(name string) []string {
	var out []string
	for _, h := range r.Headers {
		if h.Name == name {
			out = append(out, h.Value)
		}
	}
	return out
}
