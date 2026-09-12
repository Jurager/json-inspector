package command

import (
	"strings"

	"json-inspector/internal/vars"
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

// substituteForExport is what an export does with tokens. An export outlives the moment of sending,
// so it goes through the masked substitution: a secret leaves as the mask, never as its value.
func substituteForExport(text string, opts ExportOptions) string {
	if opts.Resolve == nil || opts.KeepTokens {
		return text
	}
	return vars.SubstituteMasked(text, opts.Resolve)
}

// orderedHeaders is the `Record<string, string>` the exporters iterate. Losing a header to a name
// collision would change the command, so it collapses the way a JS object does: a name that is
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

// entries is Object.entries: the current values, named in JS property order — a name that looks
// like an array index is listed before the rest, whatever position it was set in.
func (h orderedHeaders) entries() []Header {
	out := make([]Header, 0, len(h.names))
	for _, name := range jsPropOrder(h.names) {
		out = append(out, Header{Name: name, Value: h.vals[name]})
	}
	return out
}

// Export renders a request as a command for one tool. It is the inverse of Parse for the cases the
// TS is one for: what a command cannot express — a cookie file, an upload, a flag the parser
// ignored — is simply not written out.
func Export(format Format, req Request, opts ExportOptions) string {
	url := substituteForExport(req.URL, opts)
	body := substituteForExport(req.Body, opts)
	var headers orderedHeaders
	for _, h := range req.Headers {
		headers.set(substituteForExport(h.Name, opts), substituteForExport(h.Value, opts))
	}
	entries := headers.entries()

	switch format {
	case FormatCurl:
		parts := []string{"curl"}
		if req.Method != "GET" {
			parts = append(parts, "-X", req.Method)
		}
		for _, h := range entries {
			parts = append(parts, "-H", shellQuote(h.Name+": "+h.Value))
		}
		if body != "" {
			parts = append(parts, "--data-raw", shellQuote(body))
		}
		parts = append(parts, shellQuote(url))
		return strings.Join(parts, " ")

	case FormatFetch:
		init := jsValue{kind: jsObj}
		init.set("method", jsValue{kind: jsStr, str: req.Method})
		if len(entries) > 0 {
			h := jsValue{kind: jsObj}
			for _, e := range entries {
				h.set(e.Name, jsValue{kind: jsStr, str: e.Value})
			}
			init.set("headers", h)
		}
		if body != "" {
			init.set("body", jsValue{kind: jsStr, str: body})
		}
		return "fetch(" + shellQuote(url) + ", " + jsJSONStringify(init) + ")"

	case FormatWget:
		parts := []string{"wget"}
		if req.Method != "GET" {
			parts = append(parts, "--method="+req.Method)
		}
		for _, h := range entries {
			parts = append(parts, "--header="+shellQuote(h.Name+": "+h.Value))
		}
		if body != "" {
			parts = append(parts, "--body-data="+shellQuote(body))
		}
		parts = append(parts, shellQuote(url))
		return strings.Join(parts, " ")

	case FormatHTTPie:
		parts := []string{"http"}
		if body != "" {
			parts = append(parts, "--raw="+shellQuote(body))
		}
		parts = append(parts, req.Method, shellQuote(url))
		for _, h := range entries {
			parts = append(parts, shellQuote(h.Name+":"+h.Value))
		}
		return strings.Join(parts, " ")

	case FormatPowerShell:
		parts := []string{"Invoke-RestMethod -Method " + req.Method + " -Uri " + psQuote(url)}
		if len(entries) > 0 {
			pairs := make([]string, 0, len(entries))
			for _, h := range entries {
				pairs = append(pairs, psQuote(h.Name)+" = "+psQuote(h.Value))
			}
			parts = append(parts, "-Headers @{ "+strings.Join(pairs, "; ")+" }")
		}
		if body != "" {
			parts = append(parts, "-Body "+psQuote(body))
		}
		return strings.Join(parts, " ")
	}
	return ""
}
