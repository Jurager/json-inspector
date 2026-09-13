package wails

import (
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
	"json-inspector/internal/postman"
)

// A collection travels as a file, and a file has a format. This is where the formats are declared and
// where they meet the dialogs: the collections feature and the window know none of their names, so a
// new format is a package that implements the shape below plus a line in the list — no view, no
// feature, no service method learns anything.
//
// The ports are declared here rather than in the feature because this is their only consumer: what a
// format is for is opening and saving files, and files are the desktop's business.

// Reader is one format a collection can be brought in from — a Postman file today, a document of
// another tool tomorrow.
type Reader interface {
	// Label is what the format is called to the user: in the file dialog, and in the message about a
	// file that none of them could read.
	Label() string
	// Matches is the dialog's pattern for this format — "*.json", "*.yaml" — so that a file the user
	// is allowed to pick is a file something can read.
	Matches() string
	Read(data []byte) (domain.Collection, error)
}

// Writer is one format a collection can be handed over in. Being readable does not make a format
// writable: an import of an API description is not something that can be written back.
type Writer interface {
	Label() string
	// Extension is what a file of this format ends with, which is what a save dialog suggests and
	// what the writer's filter offers.
	Extension() string
	Write(name string, items []domain.CollectionNode) ([]byte, error)
}

// readers is every format the app can read, in the order they are tried.
func readers() []Reader {
	return []Reader{postmanReader{}}
}

// writers is every format the app can write. The first is what an export uses when the window does
// not name one, so the order is the order of preference.
func writers() []Writer {
	return []Writer{postmanWriter{}}
}

// writer finds the format the window asked for by the name it got from DocumentFormats.
func writer(label string) (Writer, error) {
	for _, candidate := range writers() {
		if candidate.Label() == label {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("формат %q не поддерживается: %w", label, domain.ErrNotAllowed)
}

// importFilters is what an open dialog offers: every format by name, and — when there is more than
// one — a first entry that takes them all, so a user who does not know what their file is can still
// open it.
func importFilters() []application.FileFilter {
	all := readers()
	filters := make([]application.FileFilter, 0, len(all)+1)
	patterns := make([]string, 0, len(all))
	for _, reader := range all {
		filters = append(filters, application.FileFilter{DisplayName: reader.Label(), Pattern: reader.Matches()})
		patterns = append(patterns, reader.Matches())
	}
	if len(filters) > 1 {
		filters = append([]application.FileFilter{
			{DisplayName: "Все поддерживаемые", Pattern: strings.Join(patterns, ";")},
		}, filters...)
	}
	return filters
}

// exportFilter is what a save dialog offers for one format.
func exportFilter(w Writer) []application.FileFilter {
	return []application.FileFilter{
		{DisplayName: w.Label(), Pattern: "*" + w.Extension()},
	}
}

// ---- the formats themselves, as seen by the dialogs ----------------------

// postmanReader and postmanWriter are the Postman v2.1 format behind the two shapes above. The
// package they call knows nothing about files, dialogs or this app's screens.
type postmanReader struct{}

func (postmanReader) Label() string   { return "Postman Collection" }
func (postmanReader) Matches() string { return "*.json" }

func (postmanReader) Read(data []byte) (domain.Collection, error) {
	return postman.Import(data)
}

type postmanWriter struct{}

func (postmanWriter) Label() string     { return "Postman Collection" }
func (postmanWriter) Extension() string { return ".postman_collection.json" }

func (postmanWriter) Write(name string, items []domain.CollectionNode) ([]byte, error) {
	return postman.Export(name, items)
}
