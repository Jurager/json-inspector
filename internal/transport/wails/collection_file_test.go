package wails

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"json-inspector/internal/domain"
)

// The half of an import and an export that is not a dialog: the bytes on disk. It is tested here
// because the dialog in front of it cannot be — a native file dialog is the user's, not a test's —
// and the file is the part that can be wrong in a way nobody sees until a collection is lost.
func TestCollectionFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	both := filepath.Join(dir, "collection.postman_collection.json")

	source := domain.Collection{
		Name: "Отгружено",
		Items: []domain.CollectionNode{
			{Name: "Снаружи", Position: 1, Method: "POST",
				URL: "https://api.example.com/users",
				Params: []domain.Row{
					{ID: "p1", Name: "page", Value: "2", Enabled: true},
					{ID: "p2", Name: "filter[state]", Value: "active", Enabled: false},
				}},
		},
		Children: []domain.Collection{
			{Name: "Вложенная", Position: 0, Items: []domain.CollectionNode{
				{Name: "Внутри", Position: 0, Method: "GET",
					URL: "https://api.example.com/users?page=2", Body: `{"a": 1}`,
					Headers: []domain.Row{{ID: "h1", Name: "Accept", Value: "application/json", Enabled: true}},
					Auth:    &domain.Auth{Type: domain.AuthBearer, Token: "{{token}}"}},
			}},
		},
	}

	if err := writeCollection(both, source); err != nil {
		t.Fatalf("writeCollection: %v", err)
	}

	read, err := readCollection(both)
	if err != nil {
		t.Fatalf("readCollection: %v", err)
	}
	if read.Name != "Отгружено" || len(read.Items) != 1 || len(read.Children) != 1 {
		t.Fatalf("read = %+v, want the collection that was written", read)
	}

	nested := read.Children[0]
	if nested.Name != "Вложенная" || len(nested.Items) != 1 {
		t.Fatalf("nested = %+v, want its request", nested)
	}
	request := nested.Items[0]
	if request.Method != "GET" || request.URL != "https://api.example.com/users?page=2" ||
		request.Body != `{"a": 1}` {
		t.Errorf("request = %+v, want it as it was written", request)
	}
	if len(request.Headers) != 1 || request.Headers[0].Name != "Accept" {
		t.Errorf("headers = %+v", request.Headers)
	}
	if request.Auth == nil || request.Auth.Token != "{{token}}" {
		t.Errorf("auth = %+v, want the token as text", request.Auth)
	}
	// A parameter that is switched off is not in the address, and the file is the only place it can
	// travel — losing it here would be losing it for good.
	outside := read.Items[0]
	if len(outside.Params) != 2 || outside.Params[1].Enabled {
		t.Errorf("params = %+v, want the parked row kept", outside.Params)
	}
}

func TestReadCollectionRefusesWhatIsNotOne(t *testing.T) {
	dir := t.TempDir()
	alien := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(alien, []byte(`{"openapi": "3.0.0"}`), 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	if _, err := readCollection(alien); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("a file that is not a collection = %v, want ErrNotAllowed", err)
	}
	if _, err := readCollection(filepath.Join(dir, "нет-такого.json")); err == nil {
		t.Error("a file that is not there read as a collection")
	}
}

// A name that is legal for a collection and not for a file still becomes a file name.
func TestFileNameFor(t *testing.T) {
	for name, want := range map[string]string{
		"Пользователи":    "Пользователи.postman_collection.json",
		"API/Prod: users": "API-Prod- users.postman_collection.json",
		"   ":             "collection.postman_collection.json",
		`a\b*c?d"e<f>g|h`: "a-b-c-d-e-f-g-h.postman_collection.json",
	} {
		if got := fileNameFor(name); got != want {
			t.Errorf("fileNameFor(%q) = %q, want %q", name, got, want)
		}
	}
}

// What the dialogs offer is the kind of file — JSON — and not the tool that wrote it: a Postman
// collection is a JSON file, and a second shape inside JSON must not add a second identical filter.
func TestFileKind(t *testing.T) {
	if fileKindName != "JSON" || filePattern != "*.json" {
		t.Errorf("the dialog offers %s (%s), want the file kind", fileKindName, filePattern)
	}
	if len(jsonFilter) != 1 || jsonFilter[0].Pattern != filePattern {
		t.Errorf("filter = %+v, want one entry for the kind of file", jsonFilter)
	}
	if !strings.HasSuffix(fileNameFor("Коллекция"), fileExtension) {
		t.Errorf("a suggested name does not end in %q", fileExtension)
	}
}
