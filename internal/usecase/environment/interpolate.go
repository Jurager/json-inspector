package environment

// The `{{name}}` grammar: what a token is, which names are worth reading, and what a text comes to
// once the values are in it.
//
// It lives here because it is this feature's rule and nobody else's: a name names a variable of an
// environment, and "nothing came of it" means what blocks a send. The draft, which paints tokens as
// it draws an address field, asks through the port it declares for itself rather than reading the
// grammar — so what a token is stays in one place.

import (
	"regexp"
	"strings"

	"json-inspector/internal/domain"
)

// secretMask is what a secret's value is replaced by wherever the request outlives itself: the
// preview, the record, an export.
const secretMask = "••••"

// lookup answers what a name comes to, and whether it came to anything at all.
type lookup func(name string) (domain.Resolution, bool)

// token is one `{{name}}` in a text: where it starts, where it ends, what it is called, and how it
// was written.
type token struct {
	start int
	end   int
	name  string
	raw   string
}

// nameRe is what a name may be: a letter or an underscore to begin with, then letters, digits and
// underscores.
var nameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validName(name string) bool { return nameRe.MatchString(name) }

// tokens finds every `{{name}}` worth resolving in a text. `\{{` is an escape and stays literal,
// and a `{{` that never closes is text — the rest of the line is read as it was written.
//
// The name is trimmed the way Go trims: JS's own set differs from it in two characters, U+FEFF and
// U+0085, and neither is something anybody puts inside the braces. The set is kept in the command
// lexer instead, where a BOM pasted between words is a case that really happens.
func tokens(text string) []token {
	out := []token{}
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
		name := strings.TrimSpace(text[i+2 : close])
		if !validName(name) {
			// `{{a b}}` is not a name, so it is text. The scan resumes at the second brace rather
			// than past the token, so that a real one written inside it is still found.
			i += 2
			continue
		}

		out = append(out, token{start: i, end: close + 2, name: name, raw: text[i : close+2]})
		i = close + 2
	}
	return out
}

// interpolate writes a text out with its variables filled in. maskSecrets is the difference between
// the request that goes out and everything that outlives it: the preview, an export, the record — a
// secret leaves those as its mask and nothing else does.
//
// A name that came to nothing is left as it was written. What to do about it is the caller's
// question, and missing is how it is asked.
func interpolate(text string, resolve lookup, maskSecrets bool) string {
	found := tokens(text)
	if len(found) == 0 {
		return text
	}

	var out strings.Builder
	last := 0
	for _, t := range found {
		out.WriteString(text[last:t.start])
		switch r, ok := resolve(t.name); {
		case !ok:
			out.WriteString(t.raw)
		case maskSecrets && r.Kind == domain.VariableSecret:
			out.WriteString(secretMask)
		default:
			out.WriteString(r.Value)
		}
		last = t.end
	}
	out.WriteString(text[last:])
	return out.String()
}

// missing lists the names a text mentions that came to nothing, in the order they were written and
// each of them once.
func missing(text string, resolve lookup) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range tokens(text) {
		if _, ok := resolve(t.name); ok || seen[t.name] {
			continue
		}
		seen[t.name] = true
		out = append(out, t.name)
	}
	return out
}
