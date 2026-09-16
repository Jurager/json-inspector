package httpx

import "strings"

// complete turns what a person typed into a wire address, the way a browser's address bar would:
// a bare host gets a scheme, while an address that has one — or a "{{variable}}" hole where one
// goes — is left for the engine to refuse. It sits at the one place a request leaves the process.
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

// hasScheme says whether the address already names a protocol. A host with a port reads like one
// from the left — "example.com:8080" is letters and dots up to the colon — so the digits on the
// right are what decides: a port is digits, anything else is a scheme to refuse or to send.
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

// authority is everything before the path, query or fragment: the address's RFC 3986 authority.
func authority(url string) string {
	if end := strings.IndexAny(url, "/?#"); end >= 0 {
		return url[:end]
	}
	return url
}

// isLocal covers the names that resolve to this machine by convention rather than by DNS: anything
// under ".localhost" (RFC 6761), the whole 127.0.0.0/8 block, and "::1".
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
