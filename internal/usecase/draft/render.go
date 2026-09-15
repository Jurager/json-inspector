package draft

// Rendering a request back as a command, in the five notations this app can write — one tool per
// file beside this one. Only curl is ever read back, but all five are ways of carrying a request
// out of here, written for a person to paste into their own terminal and not for a program.

import (
	"strings"

	"json-inspector/internal/domain"
)

// The reader knows one of them; the writer knows all five.
type CommandFormat string

const (
	FormatCurl       CommandFormat = "curl"
	FormatFetch      CommandFormat = "fetch"
	FormatWget       CommandFormat = "wget"
	FormatHTTPie     CommandFormat = "httpie"
	FormatPowerShell CommandFormat = "powershell"
)

// shellQuote wraps a value in single quotes, closing and reopening them around an embedded quote:
// to a POSIX shell the three pieces are one word and the quote survives as itself.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// psQuote is PowerShell's rule instead: inside single quotes a quote is written twice.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Losing a header to a name collision would change the command, so the list collapses: a name
// already there keeps its position and takes the new value.
type orderedHeaders struct {
	names []string
	vals  map[string]string
}

func (h *orderedHeaders) set(name, value string) {
	if h.vals == nil {
		h.vals = map[string]string{}
	}
	if _, seen := h.vals[name]; !seen {
		h.names = append(h.names, name)
	}
	h.vals[name] = value
}

func (h orderedHeaders) entries() []domain.HeaderPair {
	out := make([]domain.HeaderPair, 0, len(h.names))
	for _, name := range h.names {
		out = append(out, domain.HeaderPair{Name: name, Value: h.vals[name]})
	}
	return out
}

// RenderCommand writes a request out as a command for one tool. It is the inverse of ParseCommand
// for the cases a command can express at all: what one cannot — a cookie jar, an upload, a flag the
// reader ignored — is simply not written out.
func RenderCommand(format CommandFormat, seed Seed) string {
	var headers orderedHeaders
	for _, h := range seed.Headers {
		headers.set(h.Name, h.Value)
	}
	entries := headers.entries()

	url, body := seed.URL, seed.Body

	switch format {
	case FormatCurl:
		return renderCurl(url, seed.Method, entries, body)
	case FormatFetch:
		return renderFetch(url, seed.Method, entries, body)
	case FormatWget:
		return renderWget(url, seed.Method, entries, body)
	case FormatHTTPie:
		return renderHTTPie(url, seed.Method, entries, body)
	case FormatPowerShell:
		return renderPowerShell(url, seed.Method, entries, body)
	}
	return ""
}
