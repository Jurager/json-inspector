package wails

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"json-inspector/internal/domain"
	"json-inspector/internal/postman"
)

// A collection travels as a file, and reading and writing one happens on this side of the boundary: a
// collection is a document, and the window has no business holding one — it asks for an import, is
// told whether one happened, and draws the tree it gets back.
//
// The file is JSON, and what is inside it is a Postman collection. The dialog offers the kind of file
// — JSON is what a user picks, and which tool wrote it is what the file turns out to be — while the
// shape is named where it matters: in the message about a file that turned out to be something else.
const (
	fileKindName = "JSON"
	filePattern  = "*.json"
	// Postman's own ending for a collection file, so that whoever is handed one knows which tool it
	// is for before opening it.
	fileExtension = ".postman_collection.json"
)

var jsonFilter = []application.FileFilter{{DisplayName: fileKindName, Pattern: filePattern}}

// ImportFile asks for a file, reads it and writes what is in it into the tree. A nil tree is a
// cancelled dialog: nothing happened, and it is not a failure.
//
// There is one shape, and nothing is guessed: a file that is not a Postman collection is told that it
// is not. When a second shape arrives, the window offers them by name and the user says which one
// they are handing over — a file read as something it is not is worse than a wrong choice that says
// so out loud.
func (s *CollectionsService) ImportFile(ctx context.Context) ([]domain.Collection, error) {
	path, err := s.host.OpenFile("Импорт коллекции", jsonFilter...)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}

	imported, err := readCollection(path)
	if err != nil {
		return nil, err
	}
	return s.collections.Import(ctx, imported)
}

// ExportFile writes a collection — or one request, when the id names one — into a file the user
// picks. It answers whether anything was written: a cancelled save dialog is not an error, and the
// window says nothing about it.
func (s *CollectionsService) ExportFile(ctx context.Context, id string) (bool, error) {
	contents, err := s.collections.Full(ctx, id)
	if err != nil {
		return false, err
	}

	path, err := s.host.SaveFile("Экспорт коллекции", fileNameFor(contents.Name), jsonFilter...)
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil
	}
	if err := writeCollection(path, contents.Name, contents.Items); err != nil {
		return false, err
	}
	return true, nil
}

// readCollection is the file half of an import, apart from the dialog that names the file: the bytes
// on disk are a Postman collection, or they are not — and a file that is not says so itself, which is
// better than a reader that guesses at what it might be.
func readCollection(path string) (domain.Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Collection{}, fmt.Errorf("файл %s: %w", path, err)
	}
	return postman.Import(data)
}

// writeCollection is the file half of an export: a collection written where it was asked for.
func writeCollection(path string, name string, items []domain.CollectionNode) error {
	data, err := postman.Export(name, items)
	if err != nil {
		return err
	}
	// 0644 and not the database's 0600: the file is meant to be handed to someone else, and a
	// collection holds no secret — only the `{{tokens}}` a value would be filled into.
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("файл %s: %w", path, err)
	}
	return nil
}

// fileNameFor is the name a save dialog opens on: the collection's own, made safe for a file system
// and ending the way a Postman collection file does. `/` and `:` are legal in a collection's name and
// not in a file's.
func fileNameFor(name string) string {
	safe := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if safe == "" {
		safe = "collection"
	}
	return safe + fileExtension
}
