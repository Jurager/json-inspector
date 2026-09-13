package postman

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"json-inspector/internal/domain"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	return data
}

// asRequest is the fixture row that must be a request; it saves a type assertion at every use.
func asRequest(t *testing.T, node domain.CollectionNode) domain.CollectionNode {
	t.Helper()
	if node.Kind != domain.NodeRequest {
		t.Fatalf("%q is a %s, want a request", node.Name, node.Kind)
	}
	return node
}

func TestImportReadsFoldersAndRequests(t *testing.T) {
	collection, err := Import(fixture(t, "collection.json"))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if collection.Name != "Storefront" {
		t.Errorf("name = %q, want the file's", collection.Name)
	}
	if len(collection.Items) != 2 {
		t.Fatalf("items = %d, want a folder and a request", len(collection.Items))
	}

	folder := collection.Items[0]
	if folder.Kind != domain.NodeFolder || folder.Name != "Пользователи" || len(folder.Items) != 2 {
		t.Fatalf("folder = %+v, want a folder with its two requests", folder)
	}
	if folder.Position != 0 || folder.Items[1].Position != 1 {
		t.Errorf("positions = %d, %d, want the order the file is written in",
			folder.Position, folder.Items[1].Position)
	}

	list := asRequest(t, folder.Items[0])
	if list.Method != "GET" || list.URL != "https://api.example.com/users?include=author,comments&page[size]=25" {
		t.Errorf("request = %+v, want the method and the address", list)
	}
	// A row the file switched off stays switched off: it is a thing about the request.
	if len(list.Headers) != 2 || list.Headers[1].Name != "X-Debug" || list.Headers[1].Enabled {
		t.Errorf("headers = %+v, want the disabled one kept and disabled", list.Headers)
	}
	// A parameter that is not in the address is still the request's: it travels in the list beside
	// it, which is where Postman keeps it too.
	if len(list.Params) != 3 || list.Params[2].Name != "filter[state]" || list.Params[2].Enabled {
		t.Errorf("params = %+v, want the disabled one kept", list.Params)
	}
	if list.Auth == nil || list.Auth.Type != domain.AuthBearer || list.Auth.Token != "{{token}}" {
		t.Errorf("auth = %+v, want the bearer token as it was written", list.Auth)
	}

	create := asRequest(t, folder.Items[1])
	if create.Body != "{\n  \"data\": {\n    \"type\": \"users\"\n  }\n}" {
		t.Errorf("body = %q, want it as it was written", create.Body)
	}
	if len(create.Headers) != 1 || create.Headers[0].Name != "Content-Type" {
		t.Errorf("headers = %+v", create.Headers)
	}
	// An address that is a string and not an object is the same address.
	if create.URL != "https://api.example.com/users" {
		t.Errorf("url = %q", create.URL)
	}
}

// A form written as rows becomes the form it means; a body of a kind the app cannot send is left
// out rather than half-read.
func TestImportReadsAFormBody(t *testing.T) {
	collection, err := Import(fixture(t, "collection.json"))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	report := asRequest(t, collection.Items[1])
	if report.Body != "from=2026-01-01" {
		t.Errorf("body = %q, want the enabled rows of the form", report.Body)
	}
	// Basic carries two values and the chip has one field, so they are joined here.
	if report.Auth == nil || report.Auth.Type != domain.AuthBasic || report.Auth.Token != "reader:{{secret}}" {
		t.Errorf("auth = %+v, want the pair joined", report.Auth)
	}
}

func TestImportRefusesAFileThatIsNotACollection(t *testing.T) {
	for name, data := range map[string]string{
		"чужой JSON":    `{"openapi": "3.0.0"}`,
		"просто массив": `[1, 2, 3]`,
		"не JSON":       `curl -X GET https://example.com`,
	} {
		if _, err := Import([]byte(data)); !errors.Is(err, domain.ErrNotAllowed) {
			t.Errorf("%s: Import = %v, want ErrNotAllowed", name, err)
		}
	}
}

// The round trip is what makes an export worth anything: a collection written out and read back is
// the collection it was.
func TestRoundTrip(t *testing.T) {
	first, err := Import(fixture(t, "collection.json"))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	data, err := Export(first.Name, first.Items)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	second, err := Import(data)
	if err != nil {
		t.Fatalf("Import after Export: %v", err)
	}

	if second.Name != first.Name {
		t.Errorf("name = %q, want %q", second.Name, first.Name)
	}
	if len(second.Items) != len(first.Items) {
		t.Fatalf("items = %d, want %d", len(second.Items), len(first.Items))
	}
	for i := range first.Items {
		compare(t, first.Items[i], second.Items[i])
	}
	if !json.Valid(data) {
		t.Error("the export is not valid JSON")
	}
}

func compare(t *testing.T, want domain.CollectionNode, got domain.CollectionNode) {
	t.Helper()
	if got.Name != want.Name || got.Kind != want.Kind || got.Position != want.Position {
		t.Errorf("node = %+v, want %+v", got, want)
	}
	if want.Kind == domain.NodeFolder {
		if len(got.Items) != len(want.Items) {
			t.Fatalf("folder %q holds %d rows, want %d", got.Name, len(got.Items), len(want.Items))
		}
		for i := range want.Items {
			compare(t, want.Items[i], got.Items[i])
		}
		return
	}
	if got.Method != want.Method || got.URL != want.URL || got.Body != want.Body {
		t.Errorf("%q = %+v, want %+v", want.Name, got, want)
	}
	if len(got.Headers) != len(want.Headers) || len(got.Params) != len(want.Params) {
		t.Errorf("%q kept %d headers and %d params, want %d and %d",
			want.Name, len(got.Headers), len(got.Params), len(want.Headers), len(want.Params))
	}
	if (got.Auth == nil) != (want.Auth == nil) {
		t.Errorf("%q auth = %+v, want %+v", want.Name, got.Auth, want.Auth)
	} else if got.Auth != nil && *got.Auth != *want.Auth {
		t.Errorf("%q auth = %+v, want %+v", want.Name, *got.Auth, *want.Auth)
	}
}

// An export of one request is a file with one item — which is what the context menu promises.
func TestExportWritesASingleRequest(t *testing.T) {
	collection, err := Import(fixture(t, "collection.json"))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	single := asRequest(t, collection.Items[1])

	data, err := Export(single.Name, []domain.CollectionNode{single})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("the export does not read back: %v", err)
	}
	if doc.Info.Name != "Отчёт" || doc.Info.Schema != Schema {
		t.Errorf("info = %+v", doc.Info)
	}
	if len(doc.Item) != 1 || doc.Item[0].Request == nil {
		t.Fatalf("items = %+v, want the one request", doc.Item)
	}
	if doc.Item[0].Request.Method != "POST" {
		t.Errorf("method = %q", doc.Item[0].Request.Method)
	}
}

// A form body survives the trip: Postman has a shape for rows, and the app has one too.
func TestAFormBodyTravelsBothWays(t *testing.T) {
	file := []byte(`{"info":{"name":"Загрузка","schema":"` + Schema + `"},"item":[
		{"name":"Создать товар","request":{"method":"POST","url":{"raw":"https://api.example.com/products"},
		 "body":{"mode":"formdata","formdata":[
			{"key":"title","value":"Кофемолка Orion"},
			{"key":"off","value":"нет","disabled":true},
			{"key":"photo","src":"/Users/me/logo.png","type":"file"}]}}}]}`)

	collection, err := Import(file)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	node := asRequest(t, collection.Items[0])

	if node.BodyKind != domain.BodyForm {
		t.Errorf("bodyKind = %q, want a form", node.BodyKind)
	}
	if node.Body != "" {
		t.Errorf("body = %q, want the text left empty for a form", node.Body)
	}
	if len(node.Form) != 3 {
		t.Fatalf("form = %+v, want all three rows", node.Form)
	}
	if node.Form[0].Name != "title" || node.Form[0].Value != "Кофемолка Orion" || !node.Form[0].Enabled {
		t.Errorf("form[0] = %+v", node.Form[0])
	}
	if node.Form[1].Enabled {
		t.Error("a row the file marks disabled came back switched on")
	}
	// The paperclip is read from the presence of a path, which is exactly how Postman says "file".
	if !node.Form[2].File || node.Form[2].Src != "/Users/me/logo.png" {
		t.Errorf("form[2] = %+v, want the file row and its path", node.Form[2])
	}

	// And back out again, with the rows, the switch and the path where they were.
	data, err := Export(collection.Name, collection.Items)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("the export does not read back: %v", err)
	}
	out := doc.Item[0].Request.Body
	if out == nil || out.Mode != "formdata" {
		t.Fatalf("body = %+v, want a formdata one", out)
	}
	if len(out.FormData) != 3 {
		t.Fatalf("formdata = %+v, want all three rows", out.FormData)
	}
	if out.FormData[1].Disabled != true {
		t.Errorf("formdata[1] = %+v, want the switch kept", out.FormData[1])
	}
	if out.FormData[2].Src != "/Users/me/logo.png" {
		t.Errorf("formdata[2] = %+v, want the path kept", out.FormData[2])
	}
}

// A binary body is a path, and it is the only one the app can name: the bytes never enter the file.
func TestABinaryBodyTravelsBothWays(t *testing.T) {
	file := []byte(`{"info":{"name":"Файлы","schema":"` + Schema + `"},"item":[
		{"name":"Загрузить","request":{"method":"POST","url":{"raw":"https://api.example.com/files"},
		 "body":{"mode":"file","file":{"src":"/tmp/report.pdf"}}}}]}`)

	collection, err := Import(file)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	node := asRequest(t, collection.Items[0])
	if node.BodyKind != domain.BodyBinary || node.BodyFile != "/tmp/report.pdf" {
		t.Errorf("node = %+v, want a binary body and its path", node)
	}

	data, err := Export(collection.Name, collection.Items)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("the export does not read back: %v", err)
	}
	out := doc.Item[0].Request.Body
	if out == nil || out.Mode != "file" || out.File == nil || out.File.Src != "/tmp/report.pdf" {
		t.Errorf("body = %+v, want the path written back", out)
	}
}

// The three text kinds leave as raw, because Postman has nowhere to record which of them a body was.
// Writing a language this reader does not honour would be a promise the next import breaks.
func TestAJsonBodyLeavesAsRaw(t *testing.T) {
	node := domain.CollectionNode{
		Kind: domain.NodeRequest, Name: "Создать", Method: "POST",
		URL: "https://api.example.com/products", Body: `{"a": 1}`, BodyKind: domain.BodyJSON,
	}
	body := exportedBody(node)
	if body == nil || body.Mode != "raw" || body.Raw != `{"a": 1}` {
		t.Errorf("body = %+v, want raw text", body)
	}

	// A request with nothing in it has no body at all, not an empty one.
	empty := domain.CollectionNode{Kind: domain.NodeRequest, BodyKind: domain.BodyJSON}
	if body := exportedBody(empty); body != nil {
		t.Errorf("body = %+v, want none for a request with nothing in it", body)
	}
	// A file body with no file picked is the same kind of nothing.
	unpicked := domain.CollectionNode{Kind: domain.NodeRequest, BodyKind: domain.BodyBinary}
	if body := exportedBody(unpicked); body != nil {
		t.Errorf("body = %+v, want none for a binary body with no file", body)
	}
}
