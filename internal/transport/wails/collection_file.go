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

// A collection travels as a Postman v2.1 file. Reading and writing it happens on this side of the
// boundary: a collection is a document, and the window has no business holding one — it asks for an
// import, is told whether one happened, and draws the tree it gets back.

// postmanFilter is what the file dialogs offer. A file of another kind is still readable — the
// format is JSON and the dialog's filter is a convenience, not a rule.
var postmanFilter = application.FileFilter{DisplayName: "Коллекция Postman", Pattern: "*.json"}

// ImportFile asks for a file, reads it and writes what is in it into the tree. A nil tree is a
// cancelled dialog: nothing happened, and it is not a failure.
func (s *CollectionsService) ImportFile(ctx context.Context) ([]domain.Collection, error) {
	path, err := s.host.OpenFile("Импорт коллекции", postmanFilter)
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

// readCollection is the file half of an import, apart from the dialog that names the file: the bytes
// on disk are a collection, or they are not.
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

// ExportFile writes a collection — or one request, when the id names one — into a file the user
// chooses, and answers whether anything was written: a cancelled dialog is not an error, and the
// window says nothing about it.
func (s *CollectionsService) ExportFile(ctx context.Context, id string) (bool, error) {
	contents, err := s.collections.Full(ctx, id)
	if err != nil {
		return false, err
	}

	path, err := s.host.SaveFile("Экспорт коллекции", fileNameFor(contents.Name))
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

// fileNameFor is the name a save dialog opens on: the collection's own, made safe for a file system
// and marked for what it is. `/` and `:` are legal in a collection's name and not in a file's.
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
	return safe + ".postman_collection.json"
}
