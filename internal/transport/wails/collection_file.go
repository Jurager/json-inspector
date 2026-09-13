package wails

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"json-inspector/internal/domain"
)

// A collection travels as a file, and the kinds of file and the shapes inside them are declared in
// formats.go. Reading and writing happens on this side of the boundary: a collection is a document,
// and the window has no business holding one — it asks for an import, is told whether one happened,
// and draws the tree it gets back.

// DocumentFormats is what the window draws its file dialogs from: the kinds of file it can open and
// the shapes it can save a collection as. It names none of them itself, so a kind or a shape added in
// Go appears in a menu that already exists.
type DocumentFormats struct {
	Kinds   []string `json:"kinds"`
	Writers []string `json:"writers"`
}

func (s *CollectionsService) Formats(ctx context.Context) (DocumentFormats, error) {
	formats := DocumentFormats{Kinds: []string{}, Writers: []string{}}
	for _, kind := range files() {
		formats.Kinds = append(formats.Kinds, kind.Name)
		for _, w := range kind.Writers {
			formats.Writers = append(formats.Writers, w.Label())
		}
	}
	return formats, nil
}

// ImportFile asks for a file, reads it and writes what is in it into the tree. A nil tree is a
// cancelled dialog: nothing happened, and it is not a failure.
func (s *CollectionsService) ImportFile(ctx context.Context) ([]domain.Collection, error) {
	path, err := s.host.OpenFile("Импорт коллекции", importFilters()...)
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
// picks, as the shape they named. It answers whether anything was written: a cancelled save dialog is
// not an error, and the window says nothing about it.
//
// An empty shape is the first one the app can write, which is what a window that has only ever known
// about one of them passes.
func (s *CollectionsService) ExportFile(ctx context.Context, id string, format string) (bool, error) {
	chosen, err := chooseWriter(format)
	if err != nil {
		return false, err
	}

	contents, err := s.collections.Full(ctx, id)
	if err != nil {
		return false, err
	}

	path, err := s.host.SaveFile("Экспорт коллекции", fileNameFor(contents.Name, chosen.Extension()), exportFilter(chosen)...)
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil
	}
	if err := writeCollection(path, chosen, contents.Name, contents.Items); err != nil {
		return false, err
	}
	return true, nil
}

// chooseWriter is the shape an export goes out in: the one that was named, or the first the app can
// write when a window that does not know the names asks for an export.
func chooseWriter(label string) (Writer, error) {
	if label != "" {
		return writer(label)
	}
	for _, kind := range files() {
		if len(kind.Writers) > 0 {
			return kind.Writers[0], nil
		}
	}
	return nil, fmt.Errorf("ни одного формата для экспорта: %w", domain.ErrNotAllowed)
}

// readCollection is the file half of an import, apart from the dialog that names the file: the bytes
// on disk are a collection in one of the shapes the app knows, or they are not.
//
// The shapes are tried in the order they are registered and the first that reads the file wins: the
// file itself says what it is, so a user who picked one does not also have to name its shape. A file
// none of them reads says which were tried and why each refused.
func readCollection(path string) (domain.Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Collection{}, fmt.Errorf("файл %s: %w", path, err)
	}

	refusals := []string{}
	for _, kind := range files() {
		for _, reader := range kind.Readers {
			imported, err := reader.Read(data)
			if err == nil {
				return imported, nil
			}
			refusals = append(refusals, fmt.Sprintf("%s — %v", reader.Label(), errors.Unwrap(err)))
		}
	}
	return domain.Collection{}, fmt.Errorf("файл %s не читается как коллекция: %s: %w",
		path, strings.Join(refusals, "; "), domain.ErrNotAllowed)
}

// writeCollection is the file half of an export: a collection written where it was asked for, as the
// shape that was chosen.
func writeCollection(path string, w Writer, name string, items []domain.CollectionNode) error {
	data, err := w.Write(name, items)
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
// and marked with the format's extension. `/` and `:` are legal in a collection's name and not in a
// file's.
func fileNameFor(name string, extension string) string {
	safe := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if safe == "" {
		safe = "collection"
	}
	return safe + extension
}
