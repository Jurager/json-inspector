// Package vars is the `{{name}}` token grammar. It answers three questions the request fields ask
// over and over: where the tokens are, what they resolve to, and how to draw the text around them.
// A port of frontend/src/lib/vars.ts, quirks included.
package vars

import (
	"regexp"
	"strings"

	"json-inspector/internal/domain"
)

// SecretMask is what a secret looks like anywhere it is not deliberately revealed: a tooltip, the
// preview, an export.
const SecretMask = "••••"

// Resolver reports what a name stands for; the bool is false when it stands for nothing. The
// answer is a domain.Resolution so the grammar and the environments agree on one type.
type Resolver func(name string) (domain.Resolution, bool)

// Token is one `{{name}}` occurrence: half-open byte offsets into the text, the trimmed name and
// the raw span as written, which is what an unresolved token falls back to.
type Token struct {
	Start, End int
	Name       string
	Raw        string
}

// nameRe is the TS's /^[A-Za-z_][A-Za-z0-9_]*$/. It stays ASCII-only on purpose: anything else is
// payload the user pasted, so a JSON body like `{{"a": 1}}` passes through untouched.
var nameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// jsSpaceRunes is JavaScript's WhiteSpace ∪ LineTerminator set by code point. It is not Go's
// unicode.IsSpace — JS counts U+FEFF and not U+0085 — and a token name is trimmed with it, so the
// two sets would resolve `{{ x }}` differently around those characters. Written as code points
// because U+FEFF cannot appear in Go source and the spaces are otherwise invisible.
var jsSpaceRunes = []rune{
	0x0009, 0x000a, 0x000b, 0x000c, 0x000d, 0x0020, 0x00a0, 0x1680,
	0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007,
	0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff,
}

// jsSpace is that set as text, for trimming.
var jsSpace = string(jsSpaceRunes)

// jsTrim is String.prototype.trim.
func jsTrim(s string) string { return strings.Trim(s, jsSpace) }

// ValidName reports whether a name may be a variable at all — the same rule ParseTokens applies, so
// the environments screen refuses a name that the grammar would never resolve.
func ValidName(name string) bool {
	return nameRe.MatchString(name)
}

// ParseTokens finds every `{{name}}` in text, in order. Offsets are byte offsets into text: the TS
// counts UTF-16 code units, and the two agree everywhere except past non-ASCII text.
func ParseTokens(text string) []Token {
	out := []Token{}
	i := 0
	for i < len(text) {
		// A `\{{` escape is consumed whole, or its braces would open a token.
		if text[i] == '\\' && strings.HasPrefix(text[i:], `\{{`) {
			i += 3
			continue
		}
		if !strings.HasPrefix(text[i:], "{{") {
			i++
			continue
		}
		close := strings.Index(text[i+2:], "}}")
		if close == -1 {
			// Nothing ahead can close a token either, so the scan is over.
			break
		}
		close += i + 2
		name := jsTrim(text[i+2 : close])
		if !nameRe.MatchString(name) {
			// Not a name: resume after the opener, a real token may still follow.
			i += 2
			continue
		}
		out = append(out, Token{Start: i, End: close + 2, Name: name, Raw: text[i : close+2]})
		i = close + 2
	}
	return out
}

// Substitute replaces every token with its value, and leaves an unknown one exactly as written:
// Missing reports those and blocks sending, so blanking them here would hide the mistake.
func Substitute(text string, resolve Resolver) string {
	tokens := ParseTokens(text)
	if len(tokens) == 0 {
		return text
	}
	var out strings.Builder
	last := 0
	for _, t := range tokens {
		out.WriteString(text[last:t.Start])
		if r, ok := resolve(t.Name); ok {
			out.WriteString(r.Value)
		} else {
			out.WriteString(t.Raw)
		}
		last = t.End
	}
	out.WriteString(text[last:])
	return out.String()
}

// Missing lists the names that appear as tokens but resolve to nothing, deduplicated in order of
// first appearance.
func Missing(text string, resolve Resolver) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range ParseTokens(text) {
		if _, ok := resolve(t.Name); ok {
			continue
		}
		if seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		out = append(out, t.Name)
	}
	return out
}

// SubstituteMasked is substitution for anything that outlives the moment of sending — the preview,
// an export: a secret leaves as the mask, never as its value.
func SubstituteMasked(text string, resolve Resolver) string {
	tokens := ParseTokens(text)
	if len(tokens) == 0 {
		return text
	}
	var out strings.Builder
	last := 0
	for _, t := range tokens {
		out.WriteString(text[last:t.Start])
		if r, ok := resolve(t.Name); ok && r.Kind == domain.VariableSecret {
			out.WriteString(SecretMask)
		} else if ok {
			out.WriteString(r.Value)
		} else {
			out.WriteString(t.Raw)
		}
		last = t.End
	}
	out.WriteString(text[last:])
	return out.String()
}

// Segment is a run of text, either a token (TokenName set) or what surrounds one.
type Segment struct {
	Text      string
	TokenName string // empty when the segment is plain text
	Start     int
}

// Segments splits text into the runs a highlight layer paints over: concatenating the segments
// reproduces the input exactly, because the layer sits behind a real input and a dropped character
// would show up as misaligned text.
func Segments(text string) []Segment {
	out := []Segment{}
	last := 0
	for _, t := range ParseTokens(text) {
		if t.Start > last {
			out = append(out, Segment{Text: text[last:t.Start], Start: last})
		}
		out = append(out, Segment{Text: t.Raw, TokenName: t.Name, Start: t.Start})
		last = t.End
	}
	if last < len(text) {
		out = append(out, Segment{Text: text[last:], Start: last})
	}
	return out
}
