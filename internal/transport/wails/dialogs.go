package wails

// The file and browser dialogs. Every one of them is answered with "nothing" when the person closes
// it: a cancelled dialog is not a failure the window has to explain.

import (
	"errors"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The file and browser dialogs. Every one of them is answered with "nothing" when the person
// closes it: a cancelled dialog is not a failure the window has to explain.

// OpenURL puts a page in front of the person in whatever browser the system has. The app's own
// window is not that browser: a provider's sign-in page wants a real address bar and a real cookie
// jar, and the person wants to see where they are being asked to type.
//
// The address is named rawURL rather than url because this file parses addresses elsewhere, and a
// parameter of that name would shadow the package that does it.
func (h *Host) OpenURL(rawURL string) error {
	app := h.App()
	if app == nil {
		return errors.New("the window does not exist yet")
	}
	return app.Browser.OpenURL(rawURL)
}

// OpenFile asks the user for a file to read and answers with its path, or with nothing when the
// dialog was closed — closing a dialog is not a failure.
//
// The dialogs live on the Host because they are the desktop's, and a feature that needs a file asks
// for one here instead of knowing how this app talks to the system.
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

// withoutCancellation turns a closed dialog into "nothing happened". Wails answers a cancelled
// dialog with an error and not with an empty path, and the sentinel it uses lives in an internal
// package of the module — so the text is all there is to go by. Getting this wrong is not cosmetic:
// without it, closing a file dialog reports a failure the user did not have.
func withoutCancellation(err error) error {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "cancel") {
		return nil
	}
	return err
}

// SaveFile asks where to write a file, offering the name to start from and the filters the caller
// wants offered, and answers with the path the user chose — or with nothing, when they chose none.
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
		// The name is a suggestion: the dialog opens on it, and the user types over it or picks
		// another place, which is what a save dialog is for.
		Filename: suggestedName,
		Filters:  filters,
	}).PromptForSingleSelection()
	return path, withoutCancellation(err)
}
