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

	source := []domain.CollectionNode{
		{Kind: domain.NodeFolder, Name: "Папка", Position: 0, Items: []domain.CollectionNode{
			{Kind: domain.NodeRequest, Name: "Внутри", Position: 0, Method: "GET",
				URL: "https://api.example.com/users?page=2", Body: `{"a": 1}`,
				Headers: []domain.Row{{ID: "h1", Name: "Accept", Value: "application/json", Enabled: true}},
				Auth:    &domain.Auth{Type: domain.AuthBearer, Token: "{{token}}"}},
		}},
		{Kind: domain.NodeRequest, Name: "Снаружи", Position: 1, Method: "POST",
			URL: "https://api.example.com/users",
			Params: []domain.Row{
				{ID: "p1", Name: "page", Value: "2", Enabled: true},
				{ID: "p2", Name: "filter[state]", Value: "active", Enabled: false},
			}},
	}

	chosen, err := chooseWriter("")
	if err != nil {
		t.Fatalf("chooseWriter: %v", err)
	}
	if err := writeCollection(both, chosen, "Отгружено", source); err != nil {
		t.Fatalf("writeCollection: %v", err)
	}

	read, err := readCollection(both)
	if err != nil {
		t.Fatalf("readCollection: %v", err)
	}
	if read.Name != "Отгружено" || len(read.Items) != 2 {
		t.Fatalf("read = %+v, want the collection that was written", read)
	}

	inside := read.Items[0]
	if inside.Kind != domain.NodeFolder || len(inside.Items) != 1 {
		t.Fatalf("folder = %+v, want its request", inside)
	}
	request := inside.Items[0]
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
	outside := read.Items[1]
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

// A name that is legal for a collection and not for a file still becomes a file name, ending in the
// extension of the format it is written in.
func TestFileNameFor(t *testing.T) {
	for name, want := range map[string]string{
		"Пользователи":    "Пользователи.postman_collection.json",
		"API/Prod: users": "API-Prod- users.postman_collection.json",
		"   ":             "collection.postman_collection.json",
		`a\b*c?d"e<f>g|h`: "a-b-c-d-e-f-g-h.postman_collection.json",
	} {
		if got := fileNameFor(name, ".postman_collection.json"); got != want {
			t.Errorf("fileNameFor(%q) = %q, want %q", name, got, want)
		}
	}
	if got := fileNameFor("Коллекция", ".yaml"); got != "Коллекция.yaml" {
		t.Errorf("fileNameFor with another format = %q, want the format's own extension", got)
	}
}

// Which format a file is read in is the file's answer: the formats are tried in order and the first
// that reads it wins, so a window that only picked a file does not also have to name its kind. A
// format the app cannot write, and a file none of them reads, are both refused with a reason.
func TestFormats(t *testing.T) {
	if len(readers()) == 0 || len(writers()) == 0 {
		t.Fatal("the app knows no formats at all")
	}
	first, err := chooseWriter("")
	if err != nil {
		t.Fatalf("chooseWriter with nothing named: %v", err)
	}
	if first.Label() != writers()[0].Label() {
		t.Errorf("the default writer = %q, want the first one", first.Label())
	}

	if _, err := chooseWriter("Формат, которого нет"); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("an unknown format = %v, want ErrNotAllowed", err)
	}

	// One reader means the open dialog offers exactly it; the "all supported" entry would be a list
	// of one.
	filters := importFilters()
	if len(filters) != len(readers()) {
		t.Errorf("filters = %+v, want one per format while there is one format", filters)
	}
	if filters[0].DisplayName != readers()[0].Label() || filters[0].Pattern != readers()[0].Matches() {
		t.Errorf("filter = %+v, want the reader's own", filters[0])
	}
}

// A file none of the formats reads says which were tried, so a user with an unsupported file is told
// what the app can read rather than that something went wrong.
func TestReadCollectionNamesTheFormatsItTried(t *testing.T) {
	dir := t.TempDir()
	alien := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(alien, []byte("openapi: 3.0.0\n"), 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	_, err := readCollection(alien)
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("a file none of the formats reads = %v, want ErrNotAllowed", err)
	}
	for _, reader := range readers() {
		if !strings.Contains(err.Error(), reader.Label()) {
			t.Errorf("the message %q does not name %s", err.Error(), reader.Label())
		}
	}
}
