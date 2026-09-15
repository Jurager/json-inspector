package command

import (
	"fmt"
	"sort"
	"strings"
)

// JavaScript's own rules about names and spaces: which characters count as whitespace, which
// strings are array indices rather than properties, and the order JSON.stringify writes an
// object's keys in. The corpus compares output byte for byte, so these have to be the same
// rules the TypeScript used.

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
// would put tabs, newlines and non-breaking spaces into the pattern source where they are
// invisible; `\x{…}` keeps the set reviewable.
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

// jsPropOrder enumerates keys the way an ordinary JS object does: the array-index-like ones first
// in ascending order, then the rest in insertion order. It is not decoration — it decides the
// header order of a parsed request, the order a fetch init's headers come out in, and the text
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
