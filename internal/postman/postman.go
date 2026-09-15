// Package postman reads and writes Postman Collection v2.1 — the format a collection can be handed
// over in and brought back from. It is a pure package: it is given bytes and gives bytes, and it
// knows nothing about files, windows or the database.
//
// It is not a complete implementation of the format. What it understands is what the app has: a
// name, folders, and requests with a method, an address, headers, a body and an auth choice. Saved
// examples, scripts and variables are written by Postman and left alone here — a reader that kept
// them would be promising to give them back.
//
// The body is read and written in the modes the app composes — raw text, a form of rows, and a file
// named by its path. A GraphQL body is still left out: it is the one mode the app has no shape for,
// and a body half-read is worse than one it says it did not take.
package postman

import (
	"encoding/json"
	"fmt"
	"strings"

	"json-inspector/internal/domain"
)

// Schema is what a file written here says it is. Postman reads it, and so does anyone else looking
// at the JSON.
const Schema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

// document is a collection file.
type document struct {
	Info info   `json:"info"`
	Auth *auth  `json:"auth,omitempty"`
	Item []item `json:"item"`
}

type info struct {
	Name   string `json:"name"`
	Schema string `json:"schema,omitempty"`
}

// item is a row of a collection: a folder holds more, a request is the thing that gets sent. A folder
// is what this app calls a collection inside one, auth and all — the format has a place for both, and
// a group that lost its authorization on the way in would be a group that behaved differently here.
type item struct {
	Name    string   `json:"name"`
	Auth    *auth    `json:"auth,omitempty"`
	Item    []item   `json:"item,omitempty"`
	Request *request `json:"request,omitempty"`
}

type request struct {
	Method string  `json:"method"`
	Header []field `json:"header,omitempty"`
	Body   *body   `json:"body,omitempty"`
	URL    url     `json:"url"`
	Auth   *auth   `json:"auth,omitempty"`
}

// field is a key and a value, as Postman writes headers, query parameters and form rows. A row the
// file marks disabled is kept disabled: it is a thing about the request, not a comment.
type field struct {
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
	Src      string `json:"src,omitempty"`
}

// url is a string in some files and an object in others, which is why it reads itself: Postman
// writes the object, and a hand-written file usually writes the string.
type url struct {
	Raw   string  `json:"raw"`
	Query []field `json:"query,omitempty"`
}

func (u *url) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		u.Raw = raw
		return nil
	}
	// A named type, so this method is not called again for the same bytes.
	type object url
	return json.Unmarshal(data, (*object)(u))
}

type body struct {
	Mode       string     `json:"mode"`
	Raw        string     `json:"raw,omitempty"`
	URLEncoded []field    `json:"urlencoded,omitempty"`
	FormData   []field    `json:"formdata,omitempty"`
	File       *fileField `json:"file,omitempty"`
}

// fileField is a binary body: where the file is and nothing else. A path and not the bytes, which is
// what Postman keeps and what this app keeps — a collection file stays a description of the requests
// in it rather than a copy of everything they send.
type fileField struct {
	Src string `json:"src,omitempty"`
}

// auth is an authorization as Postman writes it: the scheme's name, and its fields under a key of
// that same name — `{"type":"bearer","bearer":[{"key":"token","value":"…"}]}`.
//
// The fields are a map here and not a struct because they belong to the scheme, and two of the
// schemes take different fields from the rest. What each key is called on either side is
// postmanSchemes, and it is the only place this format and ours disagree.
type auth struct {
	Type  string             `json:"type"`
	Extra map[string][]field `json:"-"`
}

func (a auth) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(a.Extra)+1)
	out["type"] = a.Type
	for name, fields := range a.Extra {
		out[name] = fields
	}
	return json.Marshal(out)
}

func (a *auth) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	a.Extra = map[string][]field{}
	for name, value := range raw {
		if name == "type" {
			if err := json.Unmarshal(value, &a.Type); err != nil {
				return err
			}
			continue
		}
		// A scheme this app does not know may carry something that is not a list of fields — Postman
		// has half a dozen of those. Refusing the whole file over one is worse than ignoring it.
		var fields []field
		if err := json.Unmarshal(value, &fields); err != nil {
			continue
		}
		a.Extra[name] = fields
	}
	return nil
}

// postmanScheme is one of this app's schemes as the file format writes it: what Postman calls the
// scheme, and what it calls each of its fields. Every disagreement between the two formats is
// collected here, which is what keeps them out of the code that walks a scheme.
type postmanScheme struct {
	Name   string
	Fields map[string]string
}

var postmanSchemes = map[domain.AuthType]postmanScheme{
	domain.AuthBearer: {Name: "bearer", Fields: map[string]string{"token": "token"}},
	domain.AuthBasic: {Name: "basic", Fields: map[string]string{
		"username": "username", "password": "password",
	}},
	domain.AuthAPIKey: {Name: "apikey", Fields: map[string]string{
		"key": "key", "value": "value", "place": "in",
	}},
	domain.AuthOAuth2: {Name: "oauth2", Fields: map[string]string{
		"grant": "grant_type", "tokenUrl": "accessTokenUrl", "clientId": "clientId",
		"clientSecret": "clientSecret", "clientAuth": "client_authentication",
		"scope": "scope", "audience": "audience", "place": "addTokenTo", "prefix": "headerPrefix",
	}},
	domain.AuthJWT: {Name: "jwt", Fields: map[string]string{
		"algorithm": "algorithm", "secret": "secret", "payload": "payload",
		"expiresIn": "exp", "place": "addTokenTo", "prefix": "headerPrefix",
	}},
	domain.AuthDigest: {Name: "digest", Fields: map[string]string{
		"username": "username", "password": "password",
	}},
	domain.AuthAWS: {Name: "awsv4", Fields: map[string]string{
		"accessKeyId": "accessKey", "secretAccessKey": "secretKey",
		"sessionToken": "sessionToken", "region": "region", "service": "service",
	}},
}

// postmanType is the app's scheme a file's auth names, and whether the app has it at all.
func postmanType(name string) (domain.AuthType, postmanScheme, bool) {
	for kind, scheme := range postmanSchemes {
		if scheme.Name == name {
			return kind, scheme, true
		}
	}
	return "", postmanScheme{}, false
}

// Import reads a collection file into the app's own shape. Ids are left empty: minting them is the
// tree's business, not this package's.
//
// A file that is not a collection is refused rather than guessed at — a JSON document with neither
// an info nor an item list is somebody else's file, and a half-imported collection is worse than a
// message saying what went wrong.
func Import(data []byte) (domain.Collection, error) {
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return domain.Collection{}, domain.Refuse(domain.CodeNotJSON, domain.ErrNotAllowed, nil)
	}
	if doc.Info.Name == "" && len(doc.Item) == 0 {
		return domain.Collection{}, domain.Refuse(domain.CodeNotPostman, domain.ErrNotAllowed, nil)
	}

	imported := domain.Collection{
		Name:        doc.Info.Name,
		Description: "",
	}
	if doc.Auth != nil {
		imported.Auth = authOf(*doc.Auth)
	}
	level(doc.Item, &imported)
	return imported, nil
}

// level reads one level of a file into the collection that holds it, keeping the order the rows are
// written in: that order is the order the tree draws them and the order a run walks them, and it is
// one list for requests and folders together.
//
// A row with a request is a request; a row with items and no request is a folder, which in this app is
// a collection inside one — a group with a name, its own authorization and its own place. The two
// kinds share one position line here, numbered by where the row stood in the file, so that the level
// merges back into the file's order on the way out.
func level(items []item, into *domain.Collection) {
	into.Items = []domain.CollectionNode{}
	into.Children = []domain.Collection{}

	for i, entry := range items {
		position := int64(i)
		if entry.Request != nil {
			into.Items = append(into.Items, requestNode(entry.Name, position, *entry.Request))
			continue
		}

		nested := domain.Collection{Name: entry.Name, Position: position}
		if entry.Auth != nil {
			nested.Auth = authOf(*entry.Auth)
		}
		level(entry.Item, &nested)
		into.Children = append(into.Children, nested)
	}
}

func requestNode(name string, position int64, from request) domain.CollectionNode {
	node := domain.CollectionNode{
		Name:     name,
		Position: position,
		Method:   strings.ToUpper(strings.TrimSpace(from.Method)),
		URL:      from.URL.Raw,
	}

	for _, header := range from.Header {
		if strings.TrimSpace(header.Key) == "" {
			continue
		}
		node.Headers = append(node.Headers, domain.Row{
			Name: header.Key, Value: header.Value, Enabled: !header.Disabled,
		})
	}
	// A disabled parameter is not in the address, so it is read from the list Postman keeps beside
	// it — the same list the export writes it back into.
	for _, query := range from.URL.Query {
		if strings.TrimSpace(query.Key) == "" {
			continue
		}
		node.Params = append(node.Params, domain.Row{
			Name: query.Key, Value: query.Value, Enabled: !query.Disabled,
		})
	}
	applyBody(&node, from.Body)
	if from.Auth != nil {
		if auth := authOf(*from.Auth); auth != nil {
			node.Auth = auth
		}
	}
	return node
}

// applyBody reads the request's body into the node, format and all.
//
// `raw` is not guessed into json or xml: the file does not say which it is, and guessing would change
// what a request that was saved elsewhere sends here. A GraphQL body is still left out rather than
// half-read — it is not a body this app can compose.
func applyBody(node *domain.CollectionNode, from *body) {
	if from == nil {
		return
	}
	switch from.Mode {
	case "raw":
		node.BodyKind = domain.BodyRaw
		node.Body = from.Raw
	case "urlencoded":
		// The app composes no urlencoded kind, so this arrives as the text it means.
		node.BodyKind = domain.BodyRaw
		parts := make([]string, 0, len(from.URLEncoded))
		for _, row := range from.URLEncoded {
			if row.Disabled {
				continue
			}
			parts = append(parts, row.Key+"="+row.Value)
		}
		node.Body = strings.Join(parts, "&")
	case "formdata":
		node.BodyKind = domain.BodyForm
		for _, row := range from.FormData {
			node.Form = append(node.Form, domain.FormRow{
				Name:    row.Key,
				Value:   row.Value,
				Src:     row.Src,
				File:    row.Src != "",
				Enabled: !row.Disabled,
			})
		}
	case "file":
		node.BodyKind = domain.BodyBinary
		if from.File != nil {
			node.BodyFile = from.File.Src
		}
	}
}

// authOf is the app's auth read out of a file's. A scheme this app does not have is left out rather
// than guessed at: an authorization that means nothing here is nothing, and a level with no auth of
// its own is one that inherits.
func authOf(from auth) *domain.Auth {
	kind, scheme, ok := postmanType(from.Type)
	if !ok {
		return nil
	}
	fields := map[string]string{}
	for key, name := range scheme.Fields {
		fields[key] = value(from.Extra[scheme.Name], name)
	}
	return &domain.Auth{Type: kind, Fields: fields}
}

func value(rows []field, key string) string {
	for _, row := range rows {
		if row.Key == key {
			return row.Value
		}
	}
	return ""
}

// Export writes what it is given as a collection file: a whole collection, or a single request when
// the collection holds one — the design exports one from the context menu, and a file with one item
// is what Postman expects to be handed.
//
// Tokens stay tokens: a `{{name}}` is written as the text it is, and no value behind it is written
// anywhere. That is not a rule this function enforces — the app never gives it a value to write.
func Export(collection domain.Collection) ([]byte, error) {
	doc := document{
		Info: info{Name: collection.Name, Schema: Schema},
		Item: exported(collection),
	}
	if collection.Auth != nil {
		doc.Auth = exportedAuth(*collection.Auth)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("collection %q could not be written: %w", collection.Name, err)
	}
	return append(data, '\n'), nil
}

// exported writes a level the way the file keeps it: the requests and the collections inside them in
// one list, in the order the tree draws them.
func exported(collection domain.Collection) []item {
	out := make([]item, 0, len(collection.Items)+len(collection.Children))
	for _, entry := range collection.Level() {
		if entry.Collection != nil {
			group := item{Name: entry.Collection.Name}
			if entry.Collection.Auth != nil {
				group.Auth = exportedAuth(*entry.Collection.Auth)
			}
			group.Item = exported(*entry.Collection)
			out = append(out, group)
			continue
		}
		out = append(out, item{Name: entry.Node.Name, Request: exportedRequest(*entry.Node)})
	}
	return out
}

func exportedRequest(node domain.CollectionNode) *request {
	out := &request{
		Method: node.Method,
		URL: url{
			Raw: node.URL,
		},
	}
	for _, header := range node.Headers {
		if strings.TrimSpace(header.Name) == "" {
			continue
		}
		out.Header = append(out.Header, field{Key: header.Name, Value: header.Value, Disabled: !header.Enabled})
	}
	// The parameters are written beside the address and not only inside it: a switched-off row is
	// not in the address, and this is the only place it can travel.
	for _, param := range node.Params {
		if strings.TrimSpace(param.Name) == "" {
			continue
		}
		out.URL.Query = append(out.URL.Query, field{Key: param.Name, Value: param.Value, Disabled: !param.Enabled})
	}
	out.Body = exportedBody(node)
	if node.Auth != nil {
		out.Auth = exportedAuth(*node.Auth)
	}
	return out
}

// exportedBody is the body as the file keeps it, in the mode that describes it.
//
// json, xml and raw all leave as `raw`: Postman has nowhere to record which of the three a body was,
// and writing an `options.raw.language` this reader does not honour would be a promise the next
// import would break. A form and a file do survive the trip, because Postman has a shape for each.
func exportedBody(node domain.CollectionNode) *body {
	switch domain.KindOf(node.BodyKind) {
	case domain.BodyForm:
		out := &body{Mode: "formdata"}
		for _, row := range node.Form {
			if strings.TrimSpace(row.Name) == "" {
				continue
			}
			out.FormData = append(out.FormData, field{
				Key: row.Name, Value: row.Value, Src: row.Src, Disabled: !row.Enabled,
			})
		}
		return out
	case domain.BodyBinary:
		if node.BodyFile == "" {
			return nil
		}
		return &body{Mode: "file", File: &fileField{Src: node.BodyFile}}
	default:
		if node.Body == "" {
			return nil
		}
		return &body{Mode: "raw", Raw: node.Body}
	}
}

// exportedAuth is the app's auth written the way the file format wants it. Every field the scheme
// declares is written, empty or not: a file is read by other people's tools, and a field that is
// missing reads as one that does not exist rather than one nobody filled in.
//
// The fields are written in the order the scheme declares them, which is the order Postman itself
// draws them in.
func exportedAuth(from domain.Auth) *auth {
	scheme, ok := postmanSchemes[from.Type]
	if !ok {
		return nil
	}
	out := &auth{Type: scheme.Name, Extra: map[string][]field{}}
	fields := []field{}
	for _, key := range domain.FieldKeys(from.Type) {
		name, ok := scheme.Fields[key]
		if !ok {
			continue
		}
		fields = append(fields, field{Key: name, Value: from.Get(key)})
	}
	out.Extra[scheme.Name] = fields
	return out
}
