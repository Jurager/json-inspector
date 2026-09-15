// Package command reads a pasted command line as a request and renders a request back as a
// command line. It is the Go side of frontend/src/lib/parseRequest.ts and
// frontend/src/lib/export.ts, ported statement for statement: internal/command/testdata was
// dumped from that implementation before it was deleted, so anything here that "improves" on
// it fails the corpus.
//
// This file is the value model those two halves share, and the reading half: JSON.parse as
// JavaScript does it, which is what a pasted `fetch(…, {body: …})` has to be read with.
package command

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

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
// and `JSON.stringify(…)` are identifiers rather than JSON and are reported as unreadable instead
// of guessed at.
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
