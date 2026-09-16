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

// A collection is a document, so the window does not hold one: it asks for an import and draws the
// tree it gets back. The dialog offers the kind of file (JSON); the shape — a Postman collection —
// is named where it matters, in the message about a file that turned out to be something else.
const (
	fileKindName = "JSON"
	filePattern  = "*.json"
	// Postman's own ending for a collection file, so that whoever is handed one knows which tool it
	// is for before opening it.
	fileExtension = ".postman_collection.json"
)

var jsonFilter = []application.FileFilter{{DisplayName: fileKindName, Pattern: filePattern}}

// ImportFile asks for a file and imports it; a nil tree is a cancelled dialog, not a failure.
// Nothing is guessed: a file that is not a Postman collection is told so. When a second shape
// arrives, the window will offer the shapes by name rather than read the file as one it is not.
func (s *CollectionsService) ImportFile(
	ctx context.Context,
	title string,
) ([]domain.Collection, error) {
	path, err := s.host.OpenFile(title, jsonFilter...)
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
// picks. False is a cancelled dialog, not an error, and the window says nothing about it.
func (s *CollectionsService) ExportFile(
	ctx context.Context,
	title string,
	id string,
) (bool, error) {
	contents, err := s.collections.Full(ctx, id)
	if err != nil {
		return false, err
	}

	path, err := s.host.SaveFile(title, fileNameFor(contents.Name), jsonFilter...)
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil
	}
	if err := writeCollection(path, contents); err != nil {
		return false, err
	}
	return true, nil
}

func readCollection(path string) (domain.Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Collection{}, fmt.Errorf("file %s: %w", path, err)
	}
	return postman.Import(data)
}

func writeCollection(path string, contents domain.Collection) error {
	data, err := postman.Export(contents)
	if err != nil {
		return err
	}
	// 0644 and not the database's 0600: the file is meant to be handed to someone else, and a
	// collection holds no secret — only the `{{tokens}}` a value would be filled into.
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("file %s: %w", path, err)
	}
	return nil
}

// `/` and `:` are legal in a collection's name and not in a file's, and Postman's own ending is
// what a save dialog should open on.
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
