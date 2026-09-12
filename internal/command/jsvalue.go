package command

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// This file holds the pieces of JavaScript the TS leans on without saying so: what counts as
// whitespace, what JSON.stringify writes, what String(x) produces, and what encodeURIComponent
// escapes. Each of them is reachable from an exported command, so the Go standard library is only
// used where it agrees exactly.

// jsSpaceRunes is JavaScript's WhiteSpace ∪ LineTerminator set by code point. It is not Go's
// unicode.IsSpace: JS counts U+FEFF (so a BOM between tokens separates them) and does not count
// U+0085. The tokenizer, the trimming and every `\s` in a ported regex use this set.
var jsSpaceRunes = []rune{
	0x0009, 0x000a, 0x000b, 0x000c, 0x000d, 0x0020, 0x00a0, 0x1680,
	0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007,
	0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff,
}

// jsSpace is that set as text, for trimming.
var jsSpace = string(jsSpaceRunes)

// jsTrim is String.prototype.trim.
func jsTrim(s string) string { return strings.Trim(s, jsSpace) }

// isJSSpace is /\s/.test(c). The TS tests one UTF-16 code unit at a time, which gives the same
// answer for every character here and for both halves of a surrogate pair.
func isJSSpace(r rune) bool { return strings.ContainsRune(jsSpace, r) }

// reClass renders code points as the body of a regexp character class. Spelling the characters out
// would put tabs, newlines and non-breaking spaces into the pattern source where they are invisible;
// `\x{…}` keeps the set reviewable.
func reClass(runes []rune) string {
	var b strings.Builder
	for _, r := range runes {
		b.WriteString(fmt.Sprintf("\\x{%04X}", r))
	}
	return b.String()
}

// jsArrayIndex reports whether a key is an array index, the keys a JS object enumerates first. A
// canonical index has no leading zeros and is below 2^32-1, so "01", "-1", "1e2" and "4294967295"
// are ordinary string keys.
func jsArrayIndex(s string) (uint32, bool) {
	if s == "0" {
		return 0, true
	}
	if len(s) == 0 || len(s) > 10 || s[0] == '0' {
		return 0, false
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		if !isASCIIDigit(s[i]) {
			return 0, false
		}
		n = n*10 + uint64(s[i]-'0')
	}
	if n > 4294967294 {
		return 0, false
	}
	return uint32(n), true
}

// jsPropOrder enumerates keys the way an ordinary JS object does: the array-index-like ones first in
// ascending order, then the rest in insertion order. It is not decoration — it decides the header
// order of a parsed request, the order a fetch init's headers come out in, and the text
// JSON.stringify writes for a body with numeric keys, all of which are compared byte for byte.
func jsPropOrder(names []string) []string {
	type numbered struct {
		name  string
		index uint32
	}
	var indices []numbered
	rest := []string{}
	for _, name := range names {
		if n, ok := jsArrayIndex(name); ok {
			indices = append(indices, numbered{name: name, index: n})
			continue
		}
		rest = append(rest, name)
	}
	sort.Slice(indices, func(a, b int) bool { return indices[a].index < indices[b].index })
	out := make([]string, 0, len(names))
	for _, k := range indices {
		out = append(out, k.name)
	}
	return append(out, rest...)
}

// jsValueKind is what a parsed JSON value is. Object members keep the order they were written in,
// because the TS goes JSON.parse → JSON.stringify and both preserve it.
type jsValueKind byte

const (
	jsNull jsValueKind = iota
	jsBool
	jsNum
	jsStr
	jsArr
	jsObj
)

// jsMember is one key/value pair of an object, in insertion order.
type jsMember struct {
	key string
	val jsValue
}

// jsValue is a JSON value in the shape JSON.parse hands over.
type jsValue struct {
	kind jsValueKind
	str  string
	num  float64
	bl   bool
	arr  []jsValue
	obj  []jsMember
}

// set assigns a member the way `obj[key] = value` does: a key keeps the position it first appeared
// in and takes the new value, which is what Object.fromEntries and Object.entries then show.
func (v *jsValue) set(key string, val jsValue) {
	for i := range v.obj {
		if v.obj[i].key == key {
			v.obj[i].val = val
			return
		}
	}
	v.obj = append(v.obj, jsMember{key: key, val: val})
}

// get is property access: absent is undefined, and so is a named property of anything that is not
// an object, which is why `init.headers` of a JSON number reads as absent.
func (v jsValue) get(key string) (jsValue, bool) {
	if v.kind != jsObj {
		return jsValue{}, false
	}
	for _, m := range v.obj {
		if m.key == key {
			return m.val, true
		}
	}
	return jsValue{}, false
}

// members is Object.entries for an object: the keys in JS property order, not the order they were
// written in.
func (v jsValue) members() []jsMember {
	if v.kind != jsObj {
		return nil
	}
	names := make([]string, 0, len(v.obj))
	for _, m := range v.obj {
		names = append(names, m.key)
	}
	out := make([]jsMember, 0, len(v.obj))
	for _, name := range jsPropOrder(names) {
		for _, m := range v.obj {
			if m.key == name {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// jsJSONParse is JSON.parse: strict, because a fetch init has to be a literal. `{ method: 'POST' }`
// and `JSON.stringify(…)` are identifiers rather than JSON and are reported as unreadable instead of
// guessed at.
func jsJSONParse(s string) (jsValue, bool) {
	r := &jsonReader{s: s}
	v, ok := r.value()
	if !ok {
		return jsValue{}, false
	}
	r.space()
	if r.i != len(r.s) {
		// JSON.parse rejects anything after the value, trailing whitespace aside.
		return jsValue{}, false
	}
	return v, true
}

// jsonReader walks the text once, in the style of JSON.parse itself.
type jsonReader struct {
	s string
	i int
}

// space skips JSON whitespace, which is four characters — not the whole JS `\s` class.
func (r *jsonReader) space() {
	for r.i < len(r.s) {
		switch r.s[r.i] {
		case ' ', '\t', '\n', '\r':
			r.i++
		default:
			return
		}
	}
}

func (r *jsonReader) value() (jsValue, bool) {
	r.space()
	if r.i >= len(r.s) {
		return jsValue{}, false
	}
	switch c := r.s[r.i]; {
	case c == '{':
		return r.object()
	case c == '[':
		return r.array()
	case c == '"':
		str, ok := r.stringLit()
		return jsValue{kind: jsStr, str: str}, ok
	case c == 't':
		if r.word("true") {
			return jsValue{kind: jsBool, bl: true}, true
		}
	case c == 'f':
		if r.word("false") {
			return jsValue{kind: jsBool}, true
		}
	case c == 'n':
		if r.word("null") {
			return jsValue{kind: jsNull}, true
		}
	case c == '-' || (c >= '0' && c <= '9'):
		return r.number()
	}
	return jsValue{}, false
}

func (r *jsonReader) word(w string) bool {
	if strings.HasPrefix(r.s[r.i:], w) {
		r.i += len(w)
		return true
	}
	return false
}

func (r *jsonReader) object() (jsValue, bool) {
	out := jsValue{kind: jsObj}
	r.i++
	r.space()
	if r.i < len(r.s) && r.s[r.i] == '}' {
		r.i++
		return out, true
	}
	for {
		r.space()
		if r.i >= len(r.s) || r.s[r.i] != '"' {
			return jsValue{}, false
		}
		key, ok := r.stringLit()
		if !ok {
			return jsValue{}, false
		}
		r.space()
		if r.i >= len(r.s) || r.s[r.i] != ':' {
			return jsValue{}, false
		}
		r.i++
		val, ok := r.value()
		if !ok {
			return jsValue{}, false
		}
		out.set(key, val)
		r.space()
		if r.i >= len(r.s) {
			return jsValue{}, false
		}
		switch r.s[r.i] {
		case ',':
			r.i++
		case '}':
			r.i++
			return out, true
		default:
			return jsValue{}, false
		}
	}
}

func (r *jsonReader) array() (jsValue, bool) {
	out := jsValue{kind: jsArr, arr: []jsValue{}}
	r.i++
	r.space()
	if r.i < len(r.s) && r.s[r.i] == ']' {
		r.i++
		return out, true
	}
	for {
		val, ok := r.value()
		if !ok {
			return jsValue{}, false
		}
		out.arr = append(out.arr, val)
		r.space()
		if r.i >= len(r.s) {
			return jsValue{}, false
		}
		switch r.s[r.i] {
		case ',':
			r.i++
		case ']':
			r.i++
			return out, true
		default:
			return jsValue{}, false
		}
	}
}

func (r *jsonReader) number() (jsValue, bool) {
	start := r.i
	if r.i < len(r.s) && r.s[r.i] == '-' {
		r.i++
	}
	if r.i < len(r.s) && r.s[r.i] == '0' {
		// A leading zero is the whole integer part: `01` is not JSON.
		r.i++
	} else if r.i < len(r.s) && r.s[r.i] >= '1' && r.s[r.i] <= '9' {
		for r.i < len(r.s) && isASCIIDigit(r.s[r.i]) {
			r.i++
		}
	} else {
		return jsValue{}, false
	}
	if r.i < len(r.s) && r.s[r.i] == '.' {
		r.i++
		if r.i >= len(r.s) || !isASCIIDigit(r.s[r.i]) {
			return jsValue{}, false
		}
		for r.i < len(r.s) && isASCIIDigit(r.s[r.i]) {
			r.i++
		}
	}
	if r.i < len(r.s) && (r.s[r.i] == 'e' || r.s[r.i] == 'E') {
		r.i++
		if r.i < len(r.s) && (r.s[r.i] == '+' || r.s[r.i] == '-') {
			r.i++
		}
		if r.i >= len(r.s) || !isASCIIDigit(r.s[r.i]) {
			return jsValue{}, false
		}
		for r.i < len(r.s) && isASCIIDigit(r.s[r.i]) {
			r.i++
		}
	}
	// JSON.parse reads the literal as a float64, so a value beyond the range is Infinity rather
	// than an error.
	f, err := strconv.ParseFloat(r.s[start:r.i], 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return jsValue{}, false
	}
	return jsValue{kind: jsNum, num: f}, true
}

func (r *jsonReader) stringLit() (string, bool) {
	if r.i >= len(r.s) || r.s[r.i] != '"' {
		return "", false
	}
	r.i++
	var b strings.Builder
	for r.i < len(r.s) {
		c := r.s[r.i]
		switch {
		case c == '"':
			r.i++
			return b.String(), true
		case c == '\\':
			r.i++
			if r.i >= len(r.s) {
				return "", false
			}
			e := r.s[r.i]
			r.i++
			switch e {
			case '"', '\\', '/':
				b.WriteByte(e)
			case 'b':
				b.WriteByte(0x08)
			case 'f':
				b.WriteByte(0x0c)
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				unit, ok := r.readHex4()
				if !ok {
					return "", false
				}
				if utf16.IsSurrogate(rune(unit)) && unit < 0xdc00 && strings.HasPrefix(r.s[r.i:], "\\u") {
					if low, ok := hex4(r.s[r.i+2:]); ok && low >= 0xdc00 && low <= 0xdfff {
						r.i += 6
						b.WriteRune(utf16.DecodeRune(rune(unit), rune(low)))
						continue
					}
				}
				b.WriteString(codeUnit(unit))
			default:
				return "", false
			}
		case c < 0x20:
			// A raw control character is not valid JSON, however it got into the paste.
			return "", false
		default:
			_, size := utf8.DecodeRuneInString(r.s[r.i:])
			b.WriteString(r.s[r.i : r.i+size])
			r.i += size
		}
	}
	return "", false
}

func (r *jsonReader) readHex4() (uint16, bool) {
	if r.i+4 > len(r.s) {
		return 0, false
	}
	unit, ok := hex4(r.s[r.i : r.i+4])
	if !ok {
		return 0, false
	}
	r.i += 4
	return unit, true
}

// jsJSONStringify renders a value the way JSON.stringify does. Go's encoder is not a substitute: it
// escapes <, > and & as < and friends, which would change an exported command.
func jsJSONStringify(v jsValue) string {
	var b strings.Builder
	writeJSJSON(&b, v)
	return b.String()
}

func writeJSJSON(b *strings.Builder, v jsValue) {
	switch v.kind {
	case jsNull:
		b.WriteString("null")
	case jsBool:
		if v.bl {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case jsNum:
		b.WriteString(jsNumberString(v.num))
	case jsStr:
		writeJSJSONString(b, v.str)
	case jsArr:
		b.WriteByte('[')
		for i, e := range v.arr {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSJSON(b, e)
		}
		b.WriteByte(']')
	case jsObj:
		b.WriteByte('{')
		for i, m := range v.members() {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSJSONString(b, m.key)
			b.WriteByte(':')
			writeJSJSON(b, m.val)
		}
		b.WriteByte('}')
	}
}

// writeJSJSONString escapes what JSON.stringify escapes and nothing else: the quote, the backslash,
// the short forms of the usual controls, and \u00xx for the rest.
func writeJSJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}

// jsToString is String(value) for a header value that is not a string. A null never reaches here —
// the TS writes the empty string for those first — but an object or an array does, and String
// renders those as `[object Object]` and a comma-joined list, not as JSON.
func jsToString(v jsValue) string {
	switch v.kind {
	case jsStr:
		return v.str
	case jsNum:
		return jsNumberString(v.num)
	case jsBool:
		if v.bl {
			return "true"
		}
		return "false"
	case jsArr:
		parts := make([]string, 0, len(v.arr))
		for _, e := range v.arr {
			// Array.prototype.join writes null and undefined as nothing at all.
			if e.kind == jsNull {
				parts = append(parts, "")
				continue
			}
			parts = append(parts, jsToString(e))
		}
		return strings.Join(parts, ",")
	default:
		return "[object Object]"
	}
}

// jsNumberString is ECMAScript's Number::toString: the shortest digits that round-trip, written
// positionally while the exponent of the first digit is in (-7, 21) and exponentially outside it.
// Go's %g has its own thresholds, so JSON.stringify output would drift on a body like 1e21.
func jsNumberString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0" // covers -0 as well, which String(-0) also writes as "0"
	case f < 0:
		return "-" + jsNumberString(-f)
	}

	e := strconv.FormatFloat(f, 'e', -1, 64)
	mantissa, exponent := e, ""
	if idx := strings.IndexByte(e, 'e'); idx >= 0 {
		mantissa, exponent = e[:idx], e[idx+1:]
	}
	digits := strings.Replace(mantissa, ".", "", 1)
	exp, _ := strconv.Atoi(exponent)
	k := len(digits)
	n := exp + 1

	switch {
	case k <= n && n <= 21:
		return digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return "0." + strings.Repeat("0", -n) + digits
	case k == 1:
		return digits + "e" + jsExponentSign(n-1)
	default:
		return digits[:1] + "." + digits[1:] + "e" + jsExponentSign(n-1)
	}
}

// jsExponentSign writes the exponent the way JS does, always with a sign.
func jsExponentSign(e int) string {
	if e < 0 {
		return "-" + strconv.Itoa(-e)
	}
	return "+" + strconv.Itoa(e)
}

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
