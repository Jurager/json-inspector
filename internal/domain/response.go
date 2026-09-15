package domain

import "sort"

type HeaderPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func PairsFromMap(headers map[string]string) []HeaderPair {
	out := make([]HeaderPair, 0, len(headers))
	for name, value := range headers {
		out = append(out, HeaderPair{Name: name, Value: value})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

type Response struct {
	Status        int          `json:"status"`
	StatusText    string       `json:"statusText"`
	Headers       []HeaderPair `json:"headers"`
	Body          string       `json:"body"`
	DurationUs    int64        `json:"durationUs"`
	ContentType   string       `json:"contentType"`
	Error         string       `json:"error,omitempty"`
	Cancelled     bool         `json:"cancelled,omitempty"`
	// SentHeaders are the headers the engine itself put on the request — the answer to a Digest
	// challenge, which cannot be computed before a request has been refused and which therefore
	// nobody above the engine could have known about. They carry no secret: a Digest response is a
	// hash of one with a nonce the server sent, and the copy they are folded into is the masked one.
	SentHeaders []HeaderPair `json:"sentHeaders,omitempty"`
	DNSUs         *int64       `json:"dnsUs,omitempty"`
	ConnectUs     *int64       `json:"connectUs,omitempty"`
	TLSUs         *int64       `json:"tlsUs,omitempty"`
	WaitUs        *int64       `json:"waitUs,omitempty"`
	DownloadUs    *int64       `json:"downloadUs,omitempty"`
	BodyTruncated bool         `json:"bodyTruncated,omitempty"`
}

func (r *Response) HeaderValues(name string) []string {
	var out []string
	for _, h := range r.Headers {
		if h.Name == name {
			out = append(out, h.Value)
		}
	}
	return out
}
