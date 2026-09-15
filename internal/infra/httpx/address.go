package httpx

import "strings"

// complete turns the address a person wrote into one the wire can take. Somebody types
// "api.example.com/articles" and means what a browser's address bar would send: the scheme is a
// detail of the wire rather than part of the address, and the field's own example writes it out
// only because an empty box has to show something.
//
// It lives here, at the one place a request leaves the process, so that every caller gets it: the
// command line, a card of a collection, a run, a script's own sendRequest.
//
// What is already an address is left alone — including one whose scheme this app cannot speak,
// where the engine's refusal says more than a rewrite would. So is an address that still has a hole
// where a variable goes: that is not a scheme that is missing.
func complete(raw string) string {
	url := strings.TrimSpace(raw)
	switch {
	case url == "", strings.Contains(url, "{{"), hasScheme(url):
		return url
	case strings.HasPrefix(url, "//"):
		// Protocol-relative: the host is named and the scheme is left out on purpose.
		return "https:" + url
	case strings.HasPrefix(url, "/"):
		// A bare path is not an address at all, whatever is put in front of it.
		return url
	case isLocal(authority(url)):
		// A development server is the one host where https is the wrong guess, and it is what a
		// browser falls back to for these names too.
		return "http://" + url
	default:
		return "https://" + url
	}
}

// hasScheme says whether the address already names a protocol: "https://…" does, and so does any
// other "word:" of the shape RFC 3986 allows.
//
// A host with a port reads like one from the left — "example.com:8080" is all letters and dots up
// to the colon — so what decides between the two is the number on the right: a port is digits, and
// everything else is left for the engine to refuse or to send.
func hasScheme(url string) bool {
	colon := strings.IndexByte(url, ':')
	if colon < 1 {
		return false
	}
	for i, r := range url[:colon] {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'):
		default:
			return false
		}
	}

	port := url[colon+1:]
	if end := strings.IndexAny(port, "/?#"); end >= 0 {
		port = port[:end]
	}
	if port == "" {
		return true
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return true
		}
	}
	return false
}

// authority is the part of an address that names the host: everything before the path, the query or
// the fragment.
func authority(url string) string {
	if end := strings.IndexAny(url, "/?#"); end >= 0 {
		return url[:end]
	}
	return url
}

// isLocal says whether the address names this machine — "localhost", "127.0.0.1", "::1" and the
// names under ".localhost" that resolve to it.
func isLocal(authority string) bool {
	host := authority
	// A port is not part of the name, and a bracketed IPv6 address keeps its brackets until here.
	if i := strings.LastIndexByte(host, ':'); i >= 0 && !strings.HasSuffix(host, "]") {
		host = host[:i]
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	return host == "localhost" || host == "::1" ||
		strings.HasSuffix(host, ".localhost") || strings.HasPrefix(host, "127.")
}
