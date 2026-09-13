package wails

import (
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
	"json-inspector/internal/postman"
)

// A collection travels as a file, and two different questions are asked about that file: what kind of
// file it is, and what is inside it.
//
// The kind is the universal thing — JSON is JSON everywhere, and a user picking a file picks a JSON
// file, not a vendor. What tells one app's export from another's is the shape of the document inside,
// and that is a question about the content: the same JSON can be a Postman collection, another tool's
// export, or neither.
//
// So the kinds are what the dialogs offer and the shapes are what the app tries and names. A shape
// knows nothing about files at all, and a new one that also speaks JSON adds no filter to the dialog.

// FileKind is a kind of file the app can open and save: what it is called, what a dialog offers for
// it, and the shapes of document that live inside it.
type FileKind struct {
	Name    string
	Pattern string
	// Extension is what a file of this kind ends with. A shape may want a longer one — a Postman
	// collection is named for what it is — and that is the shape's business, not the kind's.
	Extension string
	Readers   []Reader
	Writers   []Writer
}

// Reader is one shape of document a collection can be read from — a Postman collection today,
// another tool's export tomorrow. It knows nothing about files: which files it is tried on is the
// kind's business, and its name is said only when none of the shapes could read the file.
type Reader interface {
	Label() string
	Read(data []byte) (domain.Collection, error)
}

// Writer is one shape a collection can be written as. Being readable does not make a shape writable:
// an import of an API description is not something that can be written back.
type Writer interface {
	Label() string
	// Extension is a file name's ending for this shape — the convention of the tool it is for, so
	// that whoever is handed the file knows what it is.
	Extension() string
	Write(name string, items []domain.CollectionNode) ([]byte, error)
}

// files is every kind of file the app knows, in the order the dialogs offer them. A shape added
// inside an existing kind needs no change here.
func files() []FileKind {
	return []FileKind{{
		Name:      "JSON",
		Pattern:   "*.json",
		Extension: ".json",
		Readers:   []Reader{postmanReader{}},
		Writers:   []Writer{postmanWriter{}},
	}}
}

// writer finds the shape the window asked for by the name it got from DocumentFormats.
func writer(label string) (Writer, error) {
	for _, kind := range files() {
		for _, candidate := range kind.Writers {
			if candidate.Label() == label {
				return candidate, nil
			}
		}
	}
	return nil, fmt.Errorf("формат %q не поддерживается: %w", label, domain.ErrNotAllowed)
}

// kindOf is the file kind a shape writes, which is what a save dialog offers.
func kindOf(w Writer) FileKind {
	for _, kind := range files() {
		for _, candidate := range kind.Writers {
			if candidate.Label() == w.Label() {
				return kind
			}
		}
	}
	return FileKind{Name: "JSON", Pattern: "*.json", Extension: ".json"}
}

// importFilters is what an open dialog offers: one entry per kind of file, and — when there is more
// than one — a first entry that takes them all. The shapes inside are not offered, because they are
// not something the user can see from the outside: the file says which one it is.
func importFilters() []application.FileFilter {
	kinds := files()
	filters := make([]application.FileFilter, 0, len(kinds)+1)
	patterns := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		filters = append(filters, application.FileFilter{DisplayName: kind.Name, Pattern: kind.Pattern})
		patterns = append(patterns, kind.Pattern)
	}
	if len(filters) > 1 {
		filters = append([]application.FileFilter{
			{DisplayName: "Все поддерживаемые", Pattern: strings.Join(patterns, ";")},
		}, filters...)
	}
	return filters
}

// exportFilter is what a save dialog offers for one shape: the kind of file it is written as.
func exportFilter(w Writer) []application.FileFilter {
	kind := kindOf(w)
	return []application.FileFilter{{DisplayName: kind.Name, Pattern: kind.Pattern}}
}

// ---- the shapes themselves, as seen by the dialogs -----------------------

// postmanReader and postmanWriter are the Postman v2.1 shape behind the two forms above. The package
// they call knows nothing about files, dialogs or this app's screens.
type postmanReader struct{}

func (postmanReader) Label() string { return "Postman Collection" }

func (postmanReader) Read(data []byte) (domain.Collection, error) {
	return postman.Import(data)
}

type postmanWriter struct{}

func (postmanWriter) Label() string { return "Postman Collection" }

// A file named for what it is: Postman's own export carries this ending, so whoever is handed the
// file knows which tool it is for before opening it.
func (postmanWriter) Extension() string { return ".postman_collection.json" }

func (postmanWriter) Write(name string, items []domain.CollectionNode) ([]byte, error) {
	return postman.Export(name, items)
}
