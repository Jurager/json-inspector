package command

import "strings"

// encodeURIComponent, which is how a `fetch` template literal escapes what it interpolates.

// jsEncodeURIComponent is encodeURIComponent. url.QueryEscape is not it: that writes `+` for a
// space and leaves ! ' ( ) * alone, and either difference changes the query that is sent.
func jsEncodeURIComponent(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isURIUnreserved(c) {
			b.WriteByte(c)
			continue
		}
		// Everything else is percent-encoded byte by byte, so non-ASCII travels as UTF-8.
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

// isURIUnreserved is encodeURIComponent's keep-list.
func isURIUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '-', '_', '.', '!', '~', '*', '\'', '(', ')':
		return true
	}
	return false
}

// isASCIIDigit is JS's \d, which is ASCII only.
func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }

// hex4 reads exactly four hex digits.
func hex4(s string) (uint16, bool) {
	if len(s) < 4 {
		return 0, false
	}
	n := 0
	for i := 0; i < 4; i++ {
		d := hexDigit(s[i])
		if d < 0 {
			return 0, false
		}
		n = n*16 + d
	}
	return uint16(n), true
}

// hexDigit is the value of one hex digit, or -1.
func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
