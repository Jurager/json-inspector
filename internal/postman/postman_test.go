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

// asRequest is the fixture row that must be a request, with the index checked here.
func asRequest(t *testing.T, collection domain.Collection, i int) domain.CollectionNode {
	t.Helper()
	if i >= len(collection.Items) {
		t.Fatalf("the level holds %d requests, want one at %d", len(collection.Items), i)
	}
	return collection.Items[i]
}

func TestImportReadsNestedCollectionsAndRequests(t *testing.T) {
	collection, err := Import(fixture(t, "collection.json"))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if collection.Name != "Storefront" {
		t.Errorf("name = %q, want the file's", collection.Name)
	}

	// The file writes one list; the app keeps two, because a folder is a collection inside one and a
	// request is a row of the level it is in.
	if len(collection.Children) != 1 || len(collection.Items) != 1 {
		t.Fatalf("level = %d children and %d requests, want a folder and a request",
			len(collection.Children), len(collection.Items))
	}
	nested := collection.Children[0]
	if nested.Name != "Пользователи" || len(nested.Items) != 2 {
		t.Fatalf("nested = %+v, want the folder with its two requests", nested)
	}
	// The two share one number line, numbered by where each row stood in the file, so the level merges
	// back into the file's order rather than into two groups.
	if nested.Position != 0 || collection.Items[0].Position != 1 {
		t.Errorf("positions = %d and %d, want the order the file is written in",
			nested.Position, collection.Items[0].Position)
	}
	if order := collection.Level(); order[0].Collection == nil || order[1].Node == nil {
		t.Errorf("level = %+v, want the folder first and the request after it", order)
	}
	if nested.Items[1].Position != 1 {
		t.Errorf("position inside the folder = %d, want the order the file is written in",
			nested.Items[1].Position)
	}

	list := asRequest(t, nested, 0)
	if list.Method != "GET" ||
		list.URL != "https://api.example.com/users?include=author,comments&page[size]=25" {
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
	if list.Auth == nil || list.Auth.Type != domain.AuthBearer ||
		list.Auth.Answer("token") != "{{token}}" {
		t.Errorf("auth = %+v, want the bearer token as it was written", list.Auth)
	}

	create := asRequest(t, nested, 1)
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
	report := asRequest(t, collection, 0)
	if report.Body != "from=2026-01-01" {
		t.Errorf("body = %q, want the enabled rows of the form", report.Body)
	}
	// Basic carries two values, and both of them are the request's: a login is not a secret and a
	// password is, and they have to come back the way they were written.
	if report.Auth == nil || report.Auth.Type != domain.AuthBasic ||
		report.Auth.Answer("username") != "reader" ||
		report.Auth.Answer("password") != "{{secret}}" {
		t.Errorf("auth = %+v, want both halves of the pair", report.Auth)
	}
}

// A folder's authorization is what everything inside it inherits, and the format has a place for
// it: a group that lost its auth on the way in would be a group that behaved differently here.
func TestImportReadsTheAuthOfANestedCollection(t *testing.T) {
	file := []byte(`{"info":{"name":"Магазин","schema":"` + Schema + `"},"item":[
		{"name":"Админ","auth":{"type":"bearer","bearer":[{"key":"token","value":"{{admin}}"}]},
		 "item":[{"name":"Список",
		          "request":{"method":"GET","url":{"raw":"https://api.example.com/admins"}}}]}]}`)

	collection, err := Import(file)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(collection.Children) != 1 {
		t.Fatalf("children = %+v, want the group", collection.Children)
	}
	nested := collection.Children[0]
	if nested.Auth == nil || nested.Auth.Answer("token") != "{{admin}}" {
		t.Errorf("auth = %+v, want the group's own", nested.Auth)
	}

	// And back out again, on the group rather than on the requests inside it.
	data, err := Export(collection)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var doc collectionFile
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("the export does not read back: %v", err)
	}
	if len(doc.Item) != 1 || doc.Item[0].Auth == nil || doc.Item[0].Request != nil {
		t.Fatalf("item = %+v, want the group with its auth", doc.Item)
	}
	if doc.Item[0].Auth.Extra["bearer"][0].Value != "{{admin}}" {
		t.Errorf("auth = %+v, want the token as it was written", doc.Item[0].Auth)
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
// the collection it was, the collections inside it included.
func TestRoundTrip(t *testing.T) {
	first, err := Import(fixture(t, "collection.json"))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	data, err := Export(first)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	second, err := Import(data)
	if err != nil {
		t.Fatalf("Import after Export: %v", err)
	}

	compareCollection(t, first, second)
	if !json.Valid(data) {
		t.Error("the export is not valid JSON")
	}
}

func compareCollection(t *testing.T, want domain.Collection, got domain.Collection) {
	t.Helper()
	if got.Name != want.Name || got.Position != want.Position {
		t.Errorf("collection = %+v, want %+v", got, want)
	}
	if len(got.Items) != len(want.Items) {
		t.Fatalf("%q holds %d requests, want %d", got.Name, len(got.Items), len(want.Items))
	}
	for i := range want.Items {
		compareNode(t, want.Items[i], got.Items[i])
	}
	if len(got.Children) != len(want.Children) {
		t.Fatalf("%q holds %d collections, want %d", got.Name, len(got.Children), len(want.Children))
	}
	for i := range want.Children {
		compareCollection(t, want.Children[i], got.Children[i])
	}
}

func compareNode(t *testing.T, want domain.CollectionNode, got domain.CollectionNode) {
	t.Helper()
	if got.Name != want.Name || got.Position != want.Position {
		t.Errorf("node = %+v, want %+v", got, want)
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
	} else if got.Auth != nil && !sameAuth(*got.Auth, *want.Auth) {
		t.Errorf("%q auth = %+v, want %+v", want.Name, *got.Auth, *want.Auth)
	}
}

// sameAuth compares two authorizations the way a person would: the same scheme and the same answers
// to its fields, whether or not the map happened to be built in the same order.
func sameAuth(got domain.Auth, want domain.Auth) bool {
	if got.Type != want.Type || len(got.Fields) != len(want.Fields) {
		return false
	}
	for key, value := range want.Fields {
		if got.Answer(key) != value {
			return false
		}
	}
	return true
}

// An export of one request is a file with one item — which is what the context menu promises.
func TestExportWritesASingleRequest(t *testing.T) {
	collection, err := Import(fixture(t, "collection.json"))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	single := asRequest(t, collection, 0)

	data, err := Export(domain.Collection{Name: single.Name, Items: []domain.CollectionNode{single}})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	var doc collectionFile
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
		{"name":"Создать товар",
		 "request":{"method":"POST","url":{"raw":"https://api.example.com/products"},
		 "body":{"mode":"formdata","formdata":[
			{"key":"title","value":"Кофемолка Orion"},
			{"key":"off","value":"нет","disabled":true},
			{"key":"photo","src":"/Users/me/logo.png","type":"file"}]}}}]}`)

	collection, err := Import(file)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	node := asRequest(t, collection, 0)

	if node.BodyKind != domain.BodyForm {
		t.Errorf("bodyKind = %q, want a form", node.BodyKind)
	}
	if node.Body != "" {
		t.Errorf("body = %q, want the text left empty for a form", node.Body)
	}
	if len(node.Form) != 3 {
		t.Fatalf("form = %+v, want all three rows", node.Form)
	}
	if node.Form[0].Name != "title" || node.Form[0].Value != "Кофемолка Orion" ||
		!node.Form[0].Enabled {
		t.Errorf("form[0] = %+v", node.Form[0])
	}
	if node.Form[1].Enabled {
		t.Error("a row the file marks disabled came back switched on")
	}
	// The paperclip is read from the presence of a path, which is exactly how Postman says "file".
	if !node.Form[2].File || node.Form[2].Src != "/Users/me/logo.png" {
		t.Errorf("form[2] = %+v, want the file row and its path", node.Form[2])
	}

	data, err := Export(collection)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var doc collectionFile
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
	node := asRequest(t, collection, 0)
	if node.BodyKind != domain.BodyBinary || node.BodyFile != "/tmp/report.pdf" {
		t.Errorf("node = %+v, want a binary body and its path", node)
	}

	data, err := Export(collection)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var doc collectionFile
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("the export does not read back: %v", err)
	}
	out := doc.Item[0].Request.Body
	if out == nil || out.Mode != "file" || out.File == nil || out.File.Src != "/tmp/report.pdf" {
		t.Errorf("body = %+v, want the path written back", out)
	}
}

// The three text kinds leave as raw, because Postman has nowhere to record which of them a body
// was. Writing a language this reader does not honour would be a promise the next import breaks.
func TestAJsonBodyLeavesAsRaw(t *testing.T) {
	node := domain.CollectionNode{
		Name: "Создать", Method: "POST",
		URL: "https://api.example.com/products", Body: `{"a": 1}`, BodyKind: domain.BodyJSON,
	}
	body := exportedBody(node)
	if body == nil || body.Mode != "raw" || body.Raw != `{"a": 1}` {
		t.Errorf("body = %+v, want raw text", body)
	}

	// A request with nothing in it has no body at all, not an empty one.
	empty := domain.CollectionNode{BodyKind: domain.BodyJSON}
	if body := exportedBody(empty); body != nil {
		t.Errorf("body = %+v, want none for a request with nothing in it", body)
	}
	// A file body with no file picked is the same kind of nothing.
	unpicked := domain.CollectionNode{BodyKind: domain.BodyBinary}
	if body := exportedBody(unpicked); body != nil {
		t.Errorf("body = %+v, want none for a binary body with no file", body)
	}
}
