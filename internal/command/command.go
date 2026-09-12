// Package command reads a pasted command line as a request and renders a request back as a command
// line. It is the Go side of frontend/src/lib/parseRequest.ts and frontend/src/lib/export.ts, ported
// statement for statement: internal/command/testdata was dumped from that implementation before it
// was deleted, so anything here that "improves" on it fails the corpus.
package command

import (
	"encoding/base64"
	"regexp"
	"strings"

	"json-inspector/internal/vars"
)

// Format is the tool a command was written for. The same five are the export targets.
type Format string

const (
	FormatCurl       Format = "curl"
	FormatFetch      Format = "fetch"
	FormatWget       Format = "wget"
	FormatHTTPie     Format = "httpie"
	FormatPowerShell Format = "powershell"
)

// Header is one request header, name and value as written.
type Header struct {
	Name  string
	Value string
}

// Request is a parsed command. Headers keep the order they were written in; a repeated name keeps
// the position of its first occurrence and the value of its last, which is what the TS gets from
// collapsing them into an object.
type Request struct {
	Method  string
	URL     string
	Headers []Header
	Body    string
}

// Reason is why a command was recognised but could not be read; the UI phrases these.
type Reason string

const (
	ReasonNoURL                Reason = "no-url"
	ReasonBadQuotes            Reason = "bad-quotes"
	ReasonLeftover             Reason = "leftover"
	ReasonBadFetchInit         Reason = "bad-fetch-init"
	ReasonUnsupportedVariable  Reason = "unsupported-variable"
	ReasonUnsupportedField     Reason = "unsupported-field"
	ReasonUnsupportedMultipart Reason = "unsupported-multipart"
)

// Kind is what Parse made of the text.
type Kind string

const (
	KindNone  Kind = "none"
	KindOK    Kind = "ok"
	KindError Kind = "error"
)

// Result is a parse outcome. KindNone means "not a command, leave the field alone"; KindError
// carries the reason the UI phrases.
type Result struct {
	Kind    Kind
	Format  Format
	Request Request
	Reason  Reason
}

// ExportOptions says what an export does with `{{tokens}}`. The zero value writes them out as they
// stand, which is what the editor wants.
type ExportOptions struct {
	Resolve    vars.Resolver
	KeepTokens bool
}

// Parse reads pasted text as a request command.
func Parse(text string) Result {
	normalized := normalizeInput(text)
	if jsTrim(normalized) == "" {
		return Result{Kind: KindNone}
	}

	found, ok := detect(normalized)
	if !ok {
		return Result{Kind: KindNone}
	}

	var tokens []string
	if found.format == FormatFetch {
		// fetch is read from the raw text: its init is JSON, not shell words.
		tokens = []string{}
	} else {
		var tokOK bool
		tokens, tokOK = tokenize(found.rest, found.dialect)
		if !tokOK {
			return Result{Kind: KindError, Reason: ReasonBadQuotes}
		}
	}

	var parsed pendingRequest
	var reason Reason
	switch found.format {
	case FormatCurl:
		parsed, reason = fromCurl(tokens)
	case FormatWget:
		parsed, reason = fromWget(tokens)
	case FormatHTTPie:
		parsed, reason = fromHTTPie(tokens)
	case FormatPowerShell:
		parsed, reason = fromPowerShell(tokens)
	case FormatFetch:
		parsed, reason = fromFetch(found.rest)
	default:
		// Unreachable: detect yields only the five formats above.
		return Result{Kind: KindNone}
	}
	if reason != "" {
		return Result{Kind: KindError, Reason: reason}
	}

	req := parsed.request()
	// Two inputs the TS calls ok and hands over unusable: an empty URL pastes an empty field, an
	// empty method renders an empty chip. An ok result is a promise that a request can be built from
	// it, so both are answered here instead. Pinned, with its reasoning, by TestDeliberateDivergences.
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.URL == "" {
		return Result{Kind: KindError, Reason: ReasonNoURL}
	}
	return Result{Kind: KindOK, Format: found.format, Request: req}
}

// headerEntry is a header before folding: the TS collects entries and collapses them once, at the
// end of a parse.
type headerEntry struct {
	name  string
	value string
}

// pendingRequest is a parser's result before the headers are folded and the method defaulted.
type pendingRequest struct {
	method  string
	url     string
	entries []headerEntry
	body    string
}

// request folds the entries into the shape a caller sees.
func (p pendingRequest) request() Request {
	return Request{Method: p.method, URL: p.url, Headers: foldHeaders(p.entries), Body: p.body}
}

// defaultMethod is the TS's `method ?? (body ? 'POST' : 'GET')`: an explicit method stands even when
// it is empty, and a body implies POST.
func defaultMethod(method optString, body string) string {
	if method.set {
		return method.value
	}
	if body != "" {
		return "POST"
	}
	return "GET"
}

// foldHeaders folds entries into headers. A repeated name merges into the position of its first
// occurrence with the value of its last, which is what a JS Map keyed by the lower-cased name does.
// Cookie is the exception: those are collected apart and leave as one joined header, appended last.
func foldHeaders(entries []headerEntry) []Header {
	var order []string
	byKey := map[string]headerEntry{}
	var cookies []string
	for _, e := range entries {
		key := strings.ToLower(e.name)
		if key == "cookie" {
			// A `Cookie:` header with no value is dropped, not kept as an empty cookie.
			if e.value != "" {
				cookies = append(cookies, e.value)
			}
			continue
		}
		if _, seen := byKey[key]; !seen {
			order = append(order, key)
		}
		byKey[key] = e
	}
	out := make([]Header, 0, len(order)+1)
	// The TS hands these over inside a plain object, so they come back out in JS property order: a
	// name that looks like an array index leads, whatever position it was written in.
	for _, key := range jsPropOrder(order) {
		out = append(out, Header{Name: byKey[key].name, Value: byKey[key].value})
	}
	if len(cookies) > 0 {
		out = append(out, Header{Name: "Cookie", Value: strings.Join(cookies, "; ")})
	}
	return out
}

// headerSeparatorRe is the TS's /[:;]/: httpie writes `Name;value`, everyone else `Name: value`.
var headerSeparatorRe = regexp.MustCompile("[:;]")

// parseHeaderArg splits one header argument. A string with no separator is a name with an empty
// value rather than a mistake — `curl -H x` sends an empty `x`.
func parseHeaderArg(raw string) headerEntry {
	at := headerSeparatorRe.FindStringIndex(raw)
	if at == nil {
		return headerEntry{name: jsTrim(raw)}
	}
	return headerEntry{name: jsTrim(raw[:at[0]]), value: jsTrim(raw[at[0]+1:])}
}

// withFormType adds the form content type when a body arrived without any Content-Type. It runs
// before folding, so a Content-Type written with any casing counts.
func withFormType(entries []headerEntry, hasBody bool) []headerEntry {
	if !hasBody {
		return entries
	}
	for _, e := range entries {
		if strings.ToLower(e.name) == "content-type" {
			return entries
		}
	}
	out := make([]headerEntry, len(entries), len(entries)+1)
	copy(out, entries)
	return append(out, headerEntry{name: "Content-Type", value: "application/x-www-form-urlencoded"})
}

// basicAuth is the TS's TextEncoder + btoa chain, which is plain base64 of the UTF-8 bytes: btoa
// over a byte-per-character string encodes exactly those bytes.
func basicAuth(user, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password))
}

// basicAuthEntry is what curl's -u and httpie's --auth have in common: a value without a colon is a
// user with no password.
func basicAuthEntry(user string) headerEntry {
	colon := strings.Index(user, ":")
	if colon == -1 {
		return headerEntry{name: "Authorization", value: basicAuth(user, "")}
	}
	return headerEntry{name: "Authorization", value: basicAuth(user[:colon], user[colon+1:])}
}

// appendQuery adds a query string to a URL that may already carry one.
func appendQuery(url, query string) string {
	if query == "" {
		return url
	}
	if strings.Contains(url, "?") {
		return url + "&" + query
	}
	return url + "?" + query
}

// pickURL is the TS's pickUrl: the first positional that reads as a URL, or else the last one, so
// `curl -X POST example.com/x` still finds its URL.
func pickURL(positionals []string) string {
	for _, p := range positionals {
		if looksLikeURL(p) {
			return p
		}
	}
	if len(positionals) == 0 {
		return ""
	}
	return positionals[len(positionals)-1]
}

// hasLeftover reports positionals other than the chosen URL: the TS removes the one occurrence it
// matched and looks at what is left, so a second URL or a stray file name is an error rather than a
// silently ignored argument.
func hasLeftover(positionals []string, url string) bool {
	rest := make([]string, 0, len(positionals))
	removed := false
	for _, p := range positionals {
		if !removed && p == url {
			removed = true
			continue
		}
		rest = append(rest, p)
	}
	return len(rest) > 0
}
