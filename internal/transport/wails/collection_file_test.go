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

// What a dialog offers is the kind of file, not the shape inside it: JSON is JSON everywhere, and a
// shape added inside it — another tool's export — must not add a second identical filter. A shape the
// app cannot write, and a file none of them reads, are both refused with a reason.
func TestFormats(t *testing.T) {
	kinds := files()
	if len(kinds) == 0 {
		t.Fatal("the app knows no kinds of file at all")
	}
	shapes := 0
	for _, kind := range kinds {
		if len(kind.Readers) == 0 || len(kind.Writers) == 0 {
			t.Errorf("%s reads %d shapes and writes %d, want both", kind.Name, len(kind.Readers), len(kind.Writers))
		}
		shapes += len(kind.Readers)
	}
	if shapes == 0 {
		t.Fatal("the app knows no shapes at all")
	}

	first, err := chooseWriter("")
	if err != nil {
		t.Fatalf("chooseWriter with nothing named: %v", err)
	}
	if first.Label() != kinds[0].Writers[0].Label() {
		t.Errorf("the default writer = %q, want the first one", first.Label())
	}
	if _, err := chooseWriter("Формат, которого нет"); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("an unknown shape = %v, want ErrNotAllowed", err)
	}

	// One kind means one filter; the "all supported" entry would be a list of one.
	filters := importFilters()
	if len(filters) != len(kinds) {
		t.Errorf("filters = %+v, want one per kind while there is one kind", filters)
	}
	if filters[0].DisplayName != kinds[0].Name || filters[0].Pattern != kinds[0].Pattern {
		t.Errorf("filter = %+v, want the kind's own", filters[0])
	}

	// The filter a save dialog offers is the kind the shape is written as, while the name it suggests
	// carries the shape's own ending — the convention of the tool it is for.
	exported := exportFilter(first)
	if len(exported) != 1 || exported[0].Pattern != kinds[0].Pattern {
		t.Errorf("export filter = %+v, want the kind's pattern", exported)
	}
	if suggested := fileNameFor("Коллекция", first.Extension()); !strings.HasSuffix(suggested, first.Extension()) {
		t.Errorf("the suggested name %q does not end in %q", suggested, first.Extension())
	}
}

// A file none of the shapes reads says which were tried, so a user with an unsupported file is told
// what the app can read rather than that something went wrong.
func TestReadCollectionNamesTheShapesItTried(t *testing.T) {
	dir := t.TempDir()
	alien := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(alien, []byte(`{"openapi": "3.0.0"}`), 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	_, err := readCollection(alien)
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("a file none of the shapes reads = %v, want ErrNotAllowed", err)
	}
	for _, kind := range files() {
		for _, reader := range kind.Readers {
			if !strings.Contains(err.Error(), reader.Label()) {
				t.Errorf("the message %q does not name %s", err.Error(), reader.Label())
			}
		}
	}
}
