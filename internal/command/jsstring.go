package command

import (
	"strings"
	"unicode/utf8"
)

// A string literal as JavaScript and the shells write one: the escapes each dialect honours,
// and the two directions of `\uXXXX` — what a literal comes to, and how a code unit is
// written back out.

// readAnsiC reads $'…', where the backslash escapes are C's. Only the four the TS knows are
// translated; anything else stands for the character that follows the backslash.
func readAnsiC(input string, at int) (readResult, bool) {
	i := at + 2
	var text strings.Builder
	for i < len(input) {
		c := input[i]
		if c == '\'' {
			return readResult{text: text.String(), next: i + 1}, true
		}
		if c != '\\' {
			size := runeSize(input, i)
			text.WriteString(input[i : i+size])
			i += size
			continue
		}
		if i+1 >= len(input) {
			return readResult{}, false
		}
		nx := input[i+1]
		switch nx {
		case 'n':
			text.WriteByte('\n')
		case 't':
			text.WriteByte('\t')
		case 'r':
			text.WriteByte('\r')
		case 'u':
			// `input.slice(i + 2, i + 6)` clamps at the end of the string.
			end := i + 6
			if end > len(input) {
				end = len(input)
			}
			text.WriteString(codeUnit(ansiCodeUnit(input[i+2 : end])))
		default:
			text.WriteByte(nx)
		}
		if nx == 'u' {
			i += 6
		} else {
			i += 2
		}
	}
	return readResult{}, false
}

// jsString reads a JS string literal that is the whole argument, quotes included. `fetch(url, …)`
// is an identifier the port cannot resolve, and the TS reports it rather than guessing.
func jsString(raw string) (string, bool) {
	t := jsTrim(raw)
	if t == "" {
		return "", false
	}
	if strings.HasPrefix(t, "'") {
		r, ok := readSingle(t, 0, posixDialect)
		return r.text, ok && r.next == len(t)
	}
	if strings.HasPrefix(t, `"`) {
		r, ok := readDouble(t, 0, posixDialect)
		return r.text, ok && r.next == len(t)
	}
	return "", false
}

// runeSize is the width of the character at i, so that a reader can copy text through without
// splitting a multi-byte character. A byte that is not valid UTF-8 comes through as itself.
func runeSize(s string, i int) int {
	if s[i] < utf8.RuneSelf {
		return 1
	}
	_, size := utf8.DecodeRuneInString(s[i:])
	return size
}

// ansiCodeUnit is `parseInt(s, 16) || 0` followed by String.fromCharCode: JS skips leading
// whitespace, accepts a sign and a 0x prefix, takes the longest run of hex digits it finds, and
// turns the NaN of a failed parse into zero. fromCharCode then reads the value modulo 2^16.
func ansiCodeUnit(s string) uint16 {
	t := strings.TrimLeft(s, jsSpace)
	negative := false
	if strings.HasPrefix(t, "+") {
		t = t[1:]
	} else if strings.HasPrefix(t, "-") {
		negative = true
		t = t[1:]
	}
	if len(t) >= 2 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X') {
		t = t[2:]
	}
	n := 0
	digits := 0
	for i := 0; i < len(t); i++ {
		d := hexDigit(t[i])
		if d < 0 {
			break
		}
		n = n*16 + d
		digits++
	}
	if digits == 0 {
		return 0
	}
	if negative {
		n = -n
	}
	return uint16(n)
}

// codeUnit is String.fromCharCode: one UTF-16 code unit. A Go string cannot hold a lone surrogate,
// so one becomes U+FFFD — the only place this port cannot follow the TS, and nothing produces it
// on purpose.
func codeUnit(v uint16) string { return string(rune(v)) }
