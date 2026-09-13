// Package vars parses and resolves {{name}} variables.
package vars

import (
	"regexp"
	"strings"

	"json-inspector/internal/domain"
)

// SecretMask is used when a secret value must not be exposed.
const SecretMask = "••••"

// Resolver resolves a variable name to its value and type.
type Resolver func(name string) (domain.Resolution, bool)

// Token represents a {{name}} occurrence in text.
type Token struct {
	Start, End int
	Name       string
	Raw        string
}

// nameRe matches valid variable names.
var nameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// jsSpaceRunes contains the whitespace characters used by JavaScript trim.
var jsSpaceRunes = []rune{
	0x0009, 0x000a, 0x000b, 0x000c, 0x000d, 0x0020, 0x00a0, 0x1680,
	0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007,
	0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff,
}

var jsSpace = string(jsSpaceRunes)

func jsTrim(s string) string {
	return strings.Trim(s, jsSpace)
}

// ValidName reports whether a variable name is valid.
func ValidName(name string) bool {
	return nameRe.MatchString(name)
}

// ParseTokens finds all valid {{name}} tokens in text.
func ParseTokens(text string) []Token {
	out := []Token{}
	i := 0

	for i < len(text) {
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
			break
		}

		close += i + 2
		name := jsTrim(text[i+2 : close])

		if !nameRe.MatchString(name) {
			i += 2
			continue
		}

		out = append(out, Token{
			Start: i,
			End:   close + 2,
			Name:  name,
			Raw:   text[i : close+2],
		})

		i = close + 2
	}

	return out
}

// Substitute replaces variables with their values.
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

// Missing lists unresolved variable names in order of first appearance.
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

// SubstituteMasked replaces secrets with SecretMask.
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

// Segment is a text fragment with an optional variable name.
type Segment struct {
	Text      string
	TokenName string
	Start     int
}

// Segments splits text into plain-text and variable segments.
func Segments(text string) []Segment {
	out := []Segment{}
	last := 0

	for _, t := range ParseTokens(text) {
		if t.Start > last {
			out = append(out, Segment{
				Text:  text[last:t.Start],
				Start: last,
			})
		}

		out = append(out, Segment{
			Text:      t.Raw,
			TokenName: t.Name,
			Start:     t.Start,
		})

		last = t.End
	}

	if last < len(text) {
		out = append(out, Segment{
			Text:  text[last:],
			Start: last,
		})
	}

	return out
}
