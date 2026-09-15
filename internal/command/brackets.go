package command

// Splitting a text at a separator that is only a separator at the top level, and finding the
// bracket a given one closes: the two questions a per-tool reader asks about a nested literal.

// splitTopLevel splits on a separator that is not inside quotes or brackets, which is how a
// PowerShell hashtable and a fetch init are taken apart.
func splitTopLevel(text string, sep byte, d dialect) []string {
	out := []string{}
	depth := 0
	start := 0
	i := 0
	for i < len(text) {
		c := text[i]
		if c == '\'' || c == '"' {
			if r, ok := readQuoted(text, i, d); ok {
				i = r.next
			} else {
				i++
			}
			continue
		}
		switch {
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == sep && depth == 0:
			out = append(out, text[start:i])
			start = i + 1
		}
		i++
	}
	out = append(out, text[start:])
	return out
}

// matchBracket finds the index just past the bracket that closes the one at open. A backtick is
// skipped as a quoted pair, which is how the TS reads a PowerShell line that hides a paren in a
// regex or in a comment.
func matchBracket(text string, open int, d dialect) (int, bool) {
	depth := 0
	i := open
	for i < len(text) {
		c := text[i]
		if c == '\'' || c == '"' || c == '`' {
			if c == '`' {
				j := i + 1
				for j < len(text) && text[j] != '`' {
					if text[j] == '\\' {
						j += 2
					} else {
						j++
					}
				}
				if j >= len(text) {
					return 0, false
				}
				i = j + 1
				continue
			}
			r, ok := readQuoted(text, i, d)
			if !ok {
				return 0, false
			}
			i = r.next
			continue
		}
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
		i++
	}
	return 0, false
}
