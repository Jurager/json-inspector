package domain

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ResponseCookie is one cookie an answer set, read out of a `Set-Cookie` row. It has no id and no
// editor: nothing in the window changes it, so there is nothing to address it by.
//
// Only what the window draws is kept. The attributes a table has no column for — `SameSite` — stay
// in the header text, which the Headers tab shows whole.
type ResponseCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Expires  string `json:"expires,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
}

// CookieDateLayout is how a cookie's expiry is written down. A day is the whole of what the window
// shows, and it is the whole of what a person reading it needs.
const CookieDateLayout = "2006-01-02"

// A `Max-Age` past this is a number nobody means; the cap is here because the arithmetic below
// would otherwise overflow and date the cookie before it was set.
const maxCookieAge = int64(100 * 365 * 24 * 60 * 60)

// ParseResponseCookies reads the answer's cookies out of its headers. Rows other than `Set-Cookie`
// are skipped, and a `Set-Cookie` line that names no cookie is dropped: there is nothing to draw.
//
// `at` is when the answer arrived. A cookie whose life is a `Max-Age` is dated from there, so a
// record read back tomorrow still says the day the cookie actually dies.
func ParseResponseCookies(headers []HeaderPair, at time.Time) []ResponseCookie {
	out := []ResponseCookie{}
	for _, header := range headers {
		if !strings.EqualFold(header.Name, "Set-Cookie") {
			continue
		}
		cookie, ok := parseSetCookie(header.Value, at)
		if !ok {
			continue
		}
		out = append(out, cookie)
	}
	return out
}

// parseSetCookie reads one `Set-Cookie` line: `name=value`, then the attributes after it. A line
// with no `=` before its first `;` names nothing and answers false.
func parseSetCookie(line string, at time.Time) (ResponseCookie, bool) {
	pair, attributes, _ := strings.Cut(line, ";")
	name, value, ok := strings.Cut(pair, "=")
	if !ok || strings.TrimSpace(name) == "" {
		return ResponseCookie{}, false
	}

	cookie := ResponseCookie{Name: strings.TrimSpace(name), Value: strings.TrimSpace(value)}
	maxAge := ""
	for _, attribute := range strings.Split(attributes, ";") {
		key, value, _ := strings.Cut(strings.TrimSpace(attribute), "=")
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "domain":
			cookie.Domain = strings.TrimSpace(value)
		case "path":
			cookie.Path = strings.TrimSpace(value)
		case "expires":
			cookie.Expires = cookieDay(value)
		case "max-age":
			maxAge = strings.TrimSpace(value)
		case "secure":
			cookie.Secure = true
		case "httponly":
			cookie.HTTPOnly = true
		}
	}

	// `Max-Age` outranks `Expires` when both are there: it is the attribute a browser obeys, so it is
	// the one that says when this cookie really goes.
	if seconds, err := strconv.ParseInt(maxAge, 10, 64); err == nil {
		if seconds > maxCookieAge {
			seconds = maxCookieAge
		}
		cookie.Expires = at.Add(time.Duration(seconds) * time.Second).Format(CookieDateLayout)
	}
	return cookie, true
}

// cookieDay turns an `Expires` attribute into a date. A date nobody can read answers empty, and the
// row then draws as a session cookie — which is how a value that was never parsed reads anyway.
// The three formats are the ones HTTP itself allows, so `http.ParseTime` is the parser for them.
func cookieDay(value string) string {
	when, err := http.ParseTime(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return when.Format(CookieDateLayout)
}
