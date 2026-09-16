package updater

import "strings"

// releaseNotes turns a release's body into the lines the update window lists, one per change.
//
// The body is written by hand at release time — the workflow no longer generates it — and the
// agreement is deliberately narrow: one change per line, each opening with a bullet. Anything else
// a body can hold, a heading or a paragraph, is not a change and is left out, so that a carelessly
// written release shows fewer lines rather than a screenful of markup.
//
// Both bullet characters are accepted: `-` and `*` are the same thing to whoever is writing the
// release, and a changelog that silently came out empty because of the punctuation would be a
// failure nobody could see from the outside.
func releaseNotes(body string) []string {
	var notes []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		item, ok := strings.CutPrefix(line, "- ")
		if !ok {
			item, ok = strings.CutPrefix(line, "* ")
		}
		if !ok {
			continue
		}
		if item = strings.TrimSpace(item); item != "" {
			notes = append(notes, item)
		}
	}
	return notes
}
