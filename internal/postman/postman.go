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

type auth struct {
	Type   string  `json:"type"`
	Bearer []field `json:"bearer,omitempty"`
	Basic  []field `json:"basic,omitempty"`
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

// authOf is the app's auth, which is a type and a token. Basic is the one that does not fit: it
// carries two values, and they are joined here because the chip that edits them has one field.
func authOf(from auth) *domain.Auth {
	switch from.Type {
	case "bearer":
		return &domain.Auth{Type: domain.AuthBearer, Token: value(from.Bearer, "token")}
	case "basic":
		user, password := value(from.Basic, "username"), value(from.Basic, "password")
		return &domain.Auth{Type: domain.AuthBasic, Token: user + ":" + password}
	default:
		return nil
	}
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

func exportedAuth(from domain.Auth) *auth {
	switch from.Type {
	case domain.AuthBearer:
		return &auth{Type: "bearer", Bearer: []field{{Key: "token", Value: from.Token}}}
	case domain.AuthBasic:
		user, password, _ := strings.Cut(from.Token, ":")
		return &auth{Type: "basic", Basic: []field{{Key: "username", Value: user}, {Key: "password", Value: password}}}
	default:
		return nil
	}
}
