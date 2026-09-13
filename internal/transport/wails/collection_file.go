package wails

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"json-inspector/internal/domain"
)

// A collection travels as a file, and the formats it can travel in are listed in formats.go. Reading
// and writing happens on this side of the boundary: a collection is a document, and the window has no
// business holding one — it asks for an import, is told whether one happened, and draws the tree it
// gets back.

// DocumentFormats is what the window draws its file dialogs from: the names it can offer for an
// import and for an export. It names none of them itself, so a format added in Go appears in a menu
// that already exists.
type DocumentFormats struct {
	Readers []string `json:"readers"`
	Writers []string `json:"writers"`
}

func (s *CollectionsService) Formats(ctx context.Context) (DocumentFormats, error) {
	formats := DocumentFormats{Readers: []string{}, Writers: []string{}}
	for _, reader := range readers() {
		formats.Readers = append(formats.Readers, reader.Label())
	}
	for _, w := range writers() {
		formats.Writers = append(formats.Writers, w.Label())
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
// picks, in the format they named. It answers whether anything was written: a cancelled save dialog
// is not an error, and the window says nothing about it.
//
// An empty format is the first one the app can write, which is what a window that has only ever
// known about one of them passes.
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

// chooseWriter is the format an export goes out in: the one that was named, or the first when a
// window that does not know the names asks for an export.
func chooseWriter(label string) (Writer, error) {
	if label != "" {
		return writer(label)
	}
	all := writers()
	if len(all) == 0 {
		return nil, fmt.Errorf("ни одного формата для экспорта: %w", domain.ErrNotAllowed)
	}
	return all[0], nil
}

// readCollection is the file half of an import, apart from the dialog that names the file: the bytes
// on disk are a collection in one of the formats the app knows, or they are not.
//
// The formats are tried in the order they are registered and the first that reads the file wins: the
// file decides, so a user who picked one does not also have to name its kind. A file none of them
// reads says which were tried and why each refused.
func readCollection(path string) (domain.Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Collection{}, fmt.Errorf("файл %s: %w", path, err)
	}

	all := readers()
	refusals := make([]string, 0, len(all))
	for _, reader := range all {
		imported, err := reader.Read(data)
		if err == nil {
			return imported, nil
		}
		refusals = append(refusals, fmt.Sprintf("%s — %v", reader.Label(), errors.Unwrap(err)))
	}
	return domain.Collection{}, fmt.Errorf("файл %s не читается как коллекция: %s: %w",
		path, strings.Join(refusals, "; "), domain.ErrNotAllowed)
}

// writeCollection is the file half of an export: a collection written where it was asked for, in the
// format that was chosen.
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
