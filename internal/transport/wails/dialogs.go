package wails

import (
	"errors"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The file and browser dialogs. Every one of them is answered with "nothing" when the person
// closes it: a cancelled dialog is not a failure the window has to explain.

// OpenURL opens the page in the system's browser: a provider's sign-in page wants a real address
// bar and a real cookie jar. The address is rawURL rather than url — this file parses addresses
// elsewhere, and the package would be shadowed.
func (h *Host) OpenURL(rawURL string) error {
	app := h.App()
	if app == nil {
		return errors.New("the window does not exist yet")
	}
	return app.Browser.OpenURL(rawURL)
}

// OpenFile answers with the path, or with nothing when the dialog was closed — a cancelled dialog
// is not a failure. The dialogs live on the Host: a feature asks for a file instead of knowing how
// this app talks to the system.
func (h *Host) OpenFile(title string, filters ...application.FileFilter) (string, error) {
	app := h.App()
	if app == nil {
		return "", errors.New("the window does not exist yet")
	}
	dialog := app.Dialog.OpenFile().SetTitle(title)
	for _, filter := range filters {
		dialog = dialog.AddFilter(filter.DisplayName, filter.Pattern)
	}
	path, err := dialog.PromptForSingleSelection()
	return path, withoutCancellation(err)
}

// A closed dialog arrives as an error, not an empty path, and the sentinel Wails uses is internal
// to the module — the text is all there is to go by. Without this, closing a file dialog reports a
// failure the user did not have.
func withoutCancellation(err error) error {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "cancel") {
		return nil
	}
	return err
}

// SaveFile answers with the path the user chose, or with nothing when they chose none.
func (h *Host) SaveFile(
	title string,
	suggestedName string,
	filters ...application.FileFilter,
) (string, error) {
	app := h.App()
	if app == nil {
		return "", errors.New("the window does not exist yet")
	}
	path, err := app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:   title,
		Message: title,
		// Filename is only a suggestion: the user types over it or picks another place.
		Filename: suggestedName,
		Filters:  filters,
	}).PromptForSingleSelection()
	return path, withoutCancellation(err)
}
