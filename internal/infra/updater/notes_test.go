package updater

import (
	"slices"
	"testing"
)

// The body is written by hand at release time, so this pins what the window shows for the shapes a
// person actually writes — and, just as much, what it does not: a heading or a paragraph is not a
// change, and the release's own markup in a list of changes is worse than showing less.
func TestReleaseNotesTakesOnlyTheBullets(t *testing.T) {
	body := `## Что нового

- Инспектор диффов: подсветка внутри массивов.
* Экспорт сохраняет порядок папок.

-
Обычный абзац, который ничего не перечисляет.
-Последняя строка без пробела после дефиса.`

	want := []string{
		"Инспектор диффов: подсветка внутри массивов.",
		"Экспорт сохраняет порядок папок.",
	}
	if got := releaseNotes(body); !slices.Equal(got, want) {
		t.Errorf("releaseNotes = %q, want %q", got, want)
	}
}

// A release with nothing but prose is not an error and not a wall of text: the window shows its
// heading and an empty list, which is the honest answer — nobody wrote down what changed.
func TestReleaseNotesOnAProseOnlyBodyIsEmpty(t *testing.T) {
	if got := releaseNotes("Выпуск с исправлениями.\n\nПодробности в трекере."); got != nil {
		t.Errorf("releaseNotes = %q, want nothing", got)
	}
}

func TestReleaseNotesOnAnEmptyBody(t *testing.T) {
	if got := releaseNotes(""); got != nil {
		t.Errorf("releaseNotes = %q, want nothing", got)
	}
}
