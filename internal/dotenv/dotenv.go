// Package dotenv reads a `.env` file into entries the import dialog can offer. It is a parser and
// nothing else: deciding which entries to keep is the dialog's job.
package dotenv

import (
	"regexp"
	"strings"
)

// Entry is one line worth importing. Secret is a guess from the name, and the user confirms or
// flips it in the dialog — it is never a statement about the value.
type Entry struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// secretHint marks names that usually hold credentials: TOKEN, SECRET, PASSWORD, KEY, AUTH.
var secretHint = regexp.MustCompile(`(?i)(TOKEN|SECRET|PASSWORD|KEY|AUTH)`)

// Parse reads `KEY=value` lines, one per line. Blank lines and `#` comments are skipped, an
// optional `export ` prefix is dropped, and a fully quoted value loses its quotes. A line without
// `=` is not an assignment, so it is skipped rather than guessed at.
func Parse(text string) []Entry {
	var out []Entry

	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		body := line
		if rest, found := strings.CutPrefix(body, "export "); found {
			body = strings.TrimSpace(rest)
		}

		eq := strings.Index(body, "=")
		if eq == -1 {
			continue
		}
		name := strings.TrimSpace(body[:eq])
		if name == "" {
			continue
		}

		value := strings.TrimSpace(body[eq+1:])
		// Both quote styles, and only when the pair matches — a value that merely ends in a quote
		// keeps it, which is what an unquoted value with a stray quote looks like.
		if len(value) >= 2 {
			first, last := value[0], value[len(value)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		out = append(out, Entry{Name: name, Value: value, Secret: secretHint.MatchString(name)})
	}

	return out
}
