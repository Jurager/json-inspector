// Package har writes captured traffic as a HAR 1.2 document: the format every browser's devtools
// and every proxy exports, and the one a bug report about a request is usually read in.
//
// It writes what the app happens to know and no more. HAR has rooms for facts this app never
// measured — the connection, the server's address, the protocol version, the time spent writing the
// request — and they are left out rather than filled with a zero: a timing of zero means "it was
// instant", which is a different claim from "nobody looked". Where the format allows -1 for a
// timing that did not happen, -1 is what a phase nobody measured gets.
package har

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"json-inspector/internal/domain"
)

// Version is the HAR revision this package writes.
const Version = "1.2"

// Entry is one captured request with its bodies, which are a read of their own: the list the app
// keeps leaves bodies behind, and a HAR file without them is a file nobody can read a request in.
type Entry struct {
	Record       domain.Record
	RequestBody  string
	ResponseBody string
}

// Export writes the document. The creator is the app itself, which is what says where the file came
// from when somebody reads it a week later.
func Export(app, version string, entries []Entry) ([]byte, error) {
	doc := logFile{
		Log: log{
			Version: Version,
			Creator: creator{Name: app, Version: version},
			Entries: make([]entry, 0, len(entries)),
		},
	}

	for _, one := range entries {
		doc.Log.Entries = append(doc.Log.Entries, entryOf(one))
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	// The trailing newline is the one the collection export writes too: a file that ends without a
	// line end is a file git and diff tools complain about.
	return append(data, '\n'), nil
}

type logFile struct {
	Log log `json:"log"`
}

type log struct {
	Version string  `json:"version"`
	Creator creator `json:"creator"`
	Entries []entry `json:"entries"`
}

type creator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type entry struct {
	StartedDateTime string   `json:"startedDateTime"`
	Time            float64  `json:"time"`
	Request         request  `json:"request"`
	Response        response `json:"response"`
	Cache           struct{} `json:"cache"`
	Timings         timings  `json:"timings"`
	// The tab a request came from is the app's own fact and not HAR's: it travels under the underscore
	// the format reserves for exactly this, so a reader that knows nothing about it ignores it.
	Tab string `json:"_tab,omitempty"`
}

type request struct {
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	Cookies     []cookie     `json:"cookies"`
	Headers     []headerPair `json:"headers"`
	QueryString []nameValue  `json:"queryString"`
	PostData    *postData    `json:"postData,omitempty"`
	BodySize    int64        `json:"bodySize"`
}

type response struct {
	Status      int          `json:"status"`
	StatusText  string       `json:"statusText"`
	Cookies     []cookie     `json:"cookies"`
	Headers     []headerPair `json:"headers"`
	Content     content      `json:"content"`
	RedirectURL string       `json:"redirectURL,omitempty"`
	BodySize    int64        `json:"bodySize"`
}

type headerPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type nameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type cookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type postData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type content struct {
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
}

// Timings are milliseconds, and -1 is "this did not happen or was not measured". A captured request
// has the two phases the browser itself times and none of the rest: the page's own clock starts at
// the call, so the lookup, the connection and the handshake are inside the wait.
type timings struct {
	Blocked float64 `json:"blocked"`
	DNS     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
	SSL     float64 `json:"ssl"`
}

const unknown = -1

func entryOf(one Entry) entry {
	record := one.Record

	return entry{
		StartedDateTime: time.UnixMilli(record.StartedAt).Format(time.RFC3339Nano),
		Time:            millis(record.DurationUs),
		Request:         requestOf(record, one.RequestBody),
		Response:        responseOf(record, one.ResponseBody),
		Timings:         timingsOf(record),
		Tab:             record.TabTitle,
	}
}

func requestOf(record domain.Record, body string) request {
	out := request{
		Method:      record.Method,
		URL:         record.URL,
		Cookies:     cookiesOf(record.RequestCookies),
		Headers:     pairsOf(record.RequestHeaders),
		QueryString: queryOf(record.URL),
		BodySize:    record.RequestBytes,
	}

	if body != "" {
		if contentType := headerOf(record.RequestHeaders, "Content-Type"); contentType != "" {
			out.PostData = &postData{MimeType: contentType, Text: body}
		} else {
			out.PostData = &postData{MimeType: "text/plain", Text: body}
		}
	}
	return out
}

func responseOf(record domain.Record, body string) response {
	return response{
		Status:      record.Status,
		StatusText:  reasonPhrase(record.StatusText, record.Status),
		Cookies:     setCookiesOf(record.ResponseHeaders),
		Headers:     pairsOf(record.ResponseHeaders),
		Content:     content{Size: record.ResponseBytes, MimeType: record.ContentType, Text: body},
		RedirectURL: redirectOf(record),
		BodySize:    record.ResponseBytes,
	}
}

func timingsOf(record domain.Record) timings {
	return timings{
		Blocked: unknown,
		DNS:     phase(record.DNSUs),
		Connect: phase(record.ConnectUs),
		Send:    unknown,
		Wait:    phase(record.WaitUs),
		Receive: phase(record.DownloadUs),
		SSL:     phase(record.TLSUs),
	}
}

// phase is one measured duration in milliseconds, and -1 for a phase nobody measured: nil is how
// the store says "this did not happen", and zero would say it happened instantly.
func phase(us *int64) float64 {
	if us == nil {
		return unknown
	}
	return millis(*us)
}

func millis(us int64) float64 {
	return float64(us) / 1000
}

// reasonPhrase is the text after the code. A record this app sent kept the whole status line in the
// field — "200 OK" — and HAR asks for the reason alone, so the number is taken off the front.
func reasonPhrase(statusText string, status int) string {
	text := strings.TrimSpace(statusText)
	if text == "" {
		return ""
	}
	if prefix := strconv.Itoa(status); prefix != "0" && strings.HasPrefix(text, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(text, prefix))
	}
	return text
}

// redirectOf is where the answer points, for the answers that point anywhere. The chain itself is
// not kept — the engine follows redirects and the record is of the last one — so a hop that was
// followed is not in this file, and a 3xx that was not is.
func redirectOf(record domain.Record) string {
	if record.Status < 300 || record.Status >= 400 {
		return ""
	}
	return headerOf(record.ResponseHeaders, "Location")
}

func pairsOf(headers []domain.HeaderPair) []headerPair {
	out := make([]headerPair, 0, len(headers))
	for _, header := range headers {
		out = append(out, headerPair{Name: header.Name, Value: header.Value})
	}
	return out
}

// The answer's cookies are its Set-Cookie rows: the app keeps no jar for the other side, and for
// captured traffic a browser usually hides the header from the page, so this is often empty — which
// is a fact about the recording and not a claim that the server set nothing.
func setCookiesOf(headers []domain.HeaderPair) []cookie {
	out := []cookie{}
	for _, header := range headers {
		if !strings.EqualFold(header.Name, "Set-Cookie") {
			continue
		}
		name, rest, ok := strings.Cut(header.Value, "=")
		if !ok {
			continue
		}
		value, _, _ := strings.Cut(rest, ";")
		out = append(out, cookie{Name: strings.TrimSpace(name), Value: strings.TrimSpace(value)})
	}
	return out
}

func cookiesOf(cookies []domain.CookieRow) []cookie {
	out := make([]cookie, 0, len(cookies))
	for _, row := range cookies {
		out = append(out, cookie{Name: row.Name, Value: row.Value})
	}
	return out
}

// queryOf is the address taken apart the way HAR wants it. The query string is not stored as
// pairs — it is part of the address, which is what was actually sent — so it is parsed here, and
// an address nobody can parse is an address with no rows rather than an error: the URL is in the
// file either way.
func queryOf(raw string) []nameValue {
	parsed, err := url.Parse(raw)
	if err != nil {
		return []nameValue{}
	}

	out := []nameValue{}
	for name, values := range parsed.Query() {
		for _, value := range values {
			out = append(out, nameValue{Name: name, Value: value})
		}
	}
	return out
}

func headerOf(headers []domain.HeaderPair, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return header.Value
		}
	}
	return ""
}
