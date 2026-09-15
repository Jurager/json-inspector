// Package command reads a pasted command line as a request and renders a request back as a
// command line. It is the Go side of frontend/src/lib/parseRequest.ts and
// frontend/src/lib/export.ts, ported statement for statement: internal/command/testdata was
// dumped from that implementation before it was deleted, so anything here that "improves" on
// it fails the corpus.
//
// This file is the lexer: the dialect each tool is quoted in, and the words a line comes apart
// into. The per-tool readers are one file each beside it.
package command

import (
	"strings"
	"unicode/utf8"
)

// dialect is how one shell quotes, escapes and continues a line. Everything else about the three
// tokenizers is shared, which is why the dialect travels as a value rather than as three copies of
// the same loop.
type dialect struct {
	ps    bool // PowerShell: backtick escapes, `''` inside a single-quoted string, @{…} literals
	caret bool // cmd.exe: a caret continues a line just as a backslash does
}

var (
	posixDialect      = dialect{}
	shellDialect      = dialect{caret: true}
	powershellDialect = dialect{ps: true}
)

// optString is a string that may be absent. The TS tells `null` (the flag had no value at all) from
// `""` (its value was empty) with `??` and with truthiness, and the two decide differently — an
// empty `--url` is not a missing `--url` — so the port keeps them apart.
type optString struct {
	value string
	set   bool
}

// splitFlag separates `--flag=value`. The inline value is nil when the token carries none, and for
// a token that is not a flag at all.
func splitFlag(token string) (string, *string) {
	if !strings.HasPrefix(token, "-") {
		return token, nil
	}
	eq := strings.Index(token, "=")
	if eq > 0 {
		inline := token[eq+1:]
		return token[:eq], &inline
	}
	return token, nil
}

// take is the TS's taker(): the inline value, else the next token, else nothing. The cursor moves
// past a token that was consumed, which is why the parsers index a slice by hand.
func take(tokens []string, cursor *int, inline *string) optString {
	if inline != nil {
		return optString{value: *inline, set: true}
	}
	if *cursor+1 < len(tokens) {
		*cursor++
		return optString{value: tokens[*cursor], set: true}
	}
	return optString{}
}

// lineJoin reports where a line continuation resumes, or -1 when there is none. The joining
// character differs per shell, and the spaces and tabs before the newline go with it.
func lineJoin(input string, at int, d dialect) int {
	ch := input[at]
	var joins bool
	switch {
	case d.ps:
		joins = ch == '`'
	case d.caret:
		joins = ch == '\\' || ch == '^'
	default:
		joins = ch == '\\'
	}
	if !joins {
		return -1
	}
	j := at + 1
	for j < len(input) && (input[j] == ' ' || input[j] == '\t') {
		j++
	}
	if j < len(input) && input[j] == '\n' {
		return j + 1
	}
	return -1
}

// byteAt is `input[i]` in the TS: an index past the end reads as the zero byte, which no comparison
// in the readers matches, so the checks stay one-liners instead of bounds tests.
func byteAt(s string, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

// readResult is what every reader returns: the text it collected and where scanning resumes.
type readResult struct {
	text string
	next int
}

// readQuoted dispatches on the quote character.
func readQuoted(text string, at int, d dialect) (readResult, bool) {
	if text[at] == '\'' {
		return readSingle(text, at, d)
	}
	return readDouble(text, at, d)
}

// readSingle reads a '…' string. A doubled quote inside one is a single quote for PowerShell alone,
// while the close-escape-reopen dance a POSIX shell uses to embed a quote is understood by every
// dialect — the TS checks it outside the PowerShell branch, and a Windows paste hits it as often.
func readSingle(input string, at int, d dialect) (readResult, bool) {
	i := at + 1
	var text strings.Builder
	for i < len(input) {
		if input[i] == '\'' {
			if d.ps && byteAt(input, i+1) == '\'' {
				text.WriteByte('\'')
				i += 2
				continue
			}
			if byteAt(input, i+1) == '\\' && byteAt(input, i+2) == '\'' && byteAt(input, i+3) == '\'' {
				text.WriteByte('\'')
				i += 4
				continue
			}
			return readResult{text: text.String(), next: i + 1}, true
		}
		size := runeSize(input, i)
		text.WriteString(input[i : i+size])
		i += size
	}
	return readResult{}, false
}

// readDouble reads a "…" string. The escape character is a backslash, or a backtick in PowerShell,
// and only a quote, a backslash, a dollar and the escape character itself may be escaped: anything
// else keeps the escape character as written, which is how `"C:\Users"` survives.
func readDouble(input string, at int, d dialect) (readResult, bool) {
	esc := byte('\\')
	if d.ps {
		esc = '`'
	}
	i := at + 1
	var text strings.Builder
	for i < len(input) {
		c := input[i]
		if c == esc {
			if i+1 >= len(input) {
				// A trailing escape has nothing to escape, which reads as unclosed.
				return readResult{}, false
			}
			nx := input[i+1]
			if nx == '\n' {
				i += 2
				continue
			}
			if nx == '"' || nx == '\\' || nx == '$' || nx == '`' {
				text.WriteByte(nx)
				i += 2
				continue
			}
			text.WriteByte(c)
			i++
			continue
		}
		if c == '"' {
			return readResult{text: text.String(), next: i + 1}, true
		}
		size := runeSize(input, i)
		text.WriteString(input[i : i+size])
		i += size
	}
	return readResult{}, false
}

// readHashtable reads a PowerShell @{…} whole, quotes and nesting included, so that the header
// table arrives as one token and its inner `;` does not split the command.
func readHashtable(input string, at int) (readResult, bool) {
	i := at + 2
	depth := 1
	for i < len(input) {
		c := input[i]
		if c == '\'' || c == '"' {
			r, ok := readQuoted(input, i, powershellDialect)
			if !ok {
				return readResult{}, false
			}
			i = r.next
			continue
		}
		if c == '@' && byteAt(input, i+1) == '{' {
			depth++
			i += 2
			continue
		}
		if c == '}' {
			depth--
			i++
			if depth == 0 {
				return readResult{text: input[at:i], next: i}, true
			}
			continue
		}
		i += runeSize(input, i)
	}
	return readResult{}, false
}

// tokenize splits a command's arguments, reading quotes as it goes. The bool is false when a quote
// never closes, which the caller reports as bad-quotes rather than guessing at a split.
func tokenize(input string, d dialect) ([]string, bool) {
	out := []string{}
	i := 0
	for i < len(input) {
		if join := lineJoin(input, i, d); join != -1 {
			i = join
			continue
		}
		r, size := utf8.DecodeRuneInString(input[i:])
		if isJSSpace(r) || (!d.ps && input[i] == ';') {
			i += size
			continue
		}

		var token strings.Builder
		for i < len(input) {
			if join := lineJoin(input, i, d); join != -1 {
				i = join
				continue
			}
			size := runeSize(input, i)
			r, _ := utf8.DecodeRuneInString(input[i:])
			if isJSSpace(r) || (!d.ps && input[i] == ';') {
				break
			}
			c := input[i]
			switch {
			case c == '\'' || c == '"':
				res, ok := readQuoted(input, i, d)
				if !ok {
					return nil, false
				}
				token.WriteString(res.text)
				i = res.next
			case c == '$' && byteAt(input, i+1) == '\'':
				res, ok := readAnsiC(input, i)
				if !ok {
					return nil, false
				}
				token.WriteString(res.text)
				i = res.next
			case c == '@' && byteAt(input, i+1) == '{' && d.ps:
				res, ok := readHashtable(input, i)
				if !ok {
					return nil, false
				}
				token.WriteString(res.text)
				i = res.next
			case (c == '\\' && !d.ps) || (c == '`' && d.ps):
				if i+1 >= len(input) {
					return nil, false
				}
				token.WriteString(input[i+1 : i+1+runeSize(input, i+1)])
				i += 1 + runeSize(input, i+1)
			default:
				token.WriteString(input[i : i+size])
				i += size
			}
		}
		out = append(out, token.String())
	}
	return out, true
}
