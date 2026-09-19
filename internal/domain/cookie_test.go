package domain

import (
	"testing"
	"time"
)

// The whole of one cookie, as the table draws it: the pair, then the attributes a surface can be
// asked for.
func TestOneSetCookieLineBecomesOneCookie(t *testing.T) {
	headers := []HeaderPair{
		{Name: "Content-Type", Value: "text/html"},
		{Name: "Set-Cookie", Value: "session_id=9f3c1b7e; Domain=api.example.com; Path=/v1; " +
			"Expires=Wed, 31 Dec 2026 10:00:00 GMT; Secure; HttpOnly"},
		{Name: "ETag", Value: "abc"},
	}

	cookies := ParseResponseCookies(headers, time.UnixMilli(0))
	if len(cookies) != 1 {
		t.Fatalf("cookies = %+v, want the one Set-Cookie row", cookies)
	}
	got := cookies[0]
	want := ResponseCookie{
		Name:     "session_id",
		Value:    "9f3c1b7e",
		Domain:   "api.example.com",
		Path:     "/v1",
		Expires:  "2026-12-31",
		Secure:   true,
		HTTPOnly: true,
	}
	if got != want {
		t.Errorf("cookie = %+v, want %+v", got, want)
	}
}

// Every answer line is a cookie of its own. A map would keep the first and drop the rest, which is
// the bug the headers are a sequence for in the first place.
func TestEverySetCookieRowSurvives(t *testing.T) {
	headers := []HeaderPair{
		{Name: "Set-Cookie", Value: "a=1"},
		{Name: "set-cookie", Value: "b=2"},
		{Name: "Set-Cookie", Value: "c=3"},
	}

	cookies := ParseResponseCookies(headers, time.UnixMilli(0))
	if len(cookies) != 3 {
		t.Fatalf("cookies = %+v, want all three", cookies)
	}
	for i, want := range []string{"a", "b", "c"} {
		if cookies[i].Name != want {
			t.Errorf("cookie %d = %q, want %q", i, cookies[i].Name, want)
		}
	}
}

// The date a `Max-Age` comes to is the answer's own clock, not the reader's: a record opened a week
// later must not push the cookie's death a week out.
func TestMaxAgeIsDatedFromTheAnswer(t *testing.T) {
	arrived := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	headers := []HeaderPair{{Name: "Set-Cookie", Value: "nid=42; Max-Age=86400"}}

	cookies := ParseResponseCookies(headers, arrived)
	if len(cookies) != 1 || cookies[0].Expires != "2026-03-02" {
		t.Errorf("cookies = %+v, want the day after the answer arrived", cookies)
	}
}

// `Max-Age` is what a browser obeys, so where a line carries both it is the one that decides.
func TestMaxAgeOutranksExpires(t *testing.T) {
	arrived := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	headers := []HeaderPair{
		{Name: "Set-Cookie", Value: "nid=42; Expires=Wed, 31 Dec 2031 10:00:00 GMT; Max-Age=60"},
	}

	cookies := ParseResponseCookies(headers, arrived)
	if len(cookies) != 1 || cookies[0].Expires != "2026-03-01" {
		t.Errorf("cookies = %+v, want the Max-Age day", cookies)
	}
}

// A cookie with no life of its own is a session cookie, and the table says so by leaving the date
// empty — the row draws its own word for that.
func TestACookieWithNoLifeHasNoDate(t *testing.T) {
	headers := []HeaderPair{{Name: "Set-Cookie", Value: "locale=en-US; Path=/"}}

	cookies := ParseResponseCookies(headers, time.UnixMilli(0))
	if len(cookies) != 1 || cookies[0].Expires != "" || cookies[0].Path != "/" {
		t.Errorf("cookies = %+v, want a session cookie with its path", cookies)
	}
}

// A line that names no cookie is not a cookie: there is no name to put in the row, and an empty row
// is worse than none.
func TestALineNamingNoCookieIsDropped(t *testing.T) {
	headers := []HeaderPair{
		{Name: "Set-Cookie", Value: "no-equals-sign; Path=/"},
		{Name: "Set-Cookie", Value: "=nameless"},
	}

	if cookies := ParseResponseCookies(headers, time.UnixMilli(0)); len(cookies) != 0 {
		t.Errorf("cookies = %+v, want none: neither line names one", cookies)
	}
}

// An answer that set nothing has no cookies. The tab draws its block only for a non-empty list, so
// the empty case comes back empty rather than as a row of nothing.
func TestAnAnswerThatSetNothingHasNoCookies(t *testing.T) {
	if cookies := ParseResponseCookies(nil, time.UnixMilli(0)); len(cookies) != 0 {
		t.Errorf("cookies = %+v, want none", cookies)
	}
}
