// Package postman reads and writes Postman Collection v2.1 — the format a collection can be handed
// over in and brought back from. It is a pure package: it is given bytes and gives bytes, and it
// knows nothing about files, windows or the database.
//
// It is not a complete implementation of the format. What it understands is what the app has: a
// name, folders, and requests with a method, an address, headers, a body and an auth choice. Saved
// examples, scripts and variables are written by Postman and left alone here — a reader that kept
// them would be promising to give them back.
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
	Item []item `json:"item"`
}

type info struct {
	Name   string `json:"name"`
	Schema string `json:"schema,omitempty"`
}

// item is a row of a collection: a folder holds more, a request is the thing that gets sent.
type item struct {
	Name    string   `json:"name"`
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
	Mode       string  `json:"mode"`
	Raw        string  `json:"raw,omitempty"`
	URLEncoded []field `json:"urlencoded,omitempty"`
	FormData   []field `json:"formdata,omitempty"`
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
		return domain.Collection{}, fmt.Errorf("файл не читается как JSON: %w", domain.ErrNotAllowed)
	}
	if doc.Info.Name == "" && len(doc.Item) == 0 {
		return domain.Collection{}, fmt.Errorf(
			"это не коллекция Postman: в файле нет ни info, ни item: %w", domain.ErrNotAllowed)
	}

	return domain.Collection{
		Name:        doc.Info.Name,
		Description: "",
		Items:       nodes(doc.Item),
	}, nil
}

// nodes reads a list of rows, keeping the order they are written in: that order is the order the
// tree draws them and the order a run walks them.
func nodes(items []item) []domain.CollectionNode {
	out := make([]domain.CollectionNode, 0, len(items))
	for i, entry := range items {
		node := domain.CollectionNode{
			Name:     entry.Name,
			Position: int64(i),
			Kind:     domain.NodeFolder,
			Items:    []domain.CollectionNode{},
		}
		if entry.Request != nil {
			node = requestNode(node, *entry.Request)
		} else {
			node.Items = nodes(entry.Item)
		}
		out = append(out, node)
	}
	return out
}

func requestNode(node domain.CollectionNode, from request) domain.CollectionNode {
	node.Kind = domain.NodeRequest
	node.Method = strings.ToUpper(strings.TrimSpace(from.Method))
	node.URL = from.URL.Raw
	node.Items = nil

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
	node.Body = bodyText(from.Body)
	if from.Auth != nil {
		if auth := authOf(*from.Auth); auth != nil {
			node.Auth = auth
		}
	}
	return node
}

// bodyText is the request's body as one text, which is the only shape this app composes: a form
// written as rows becomes the form it means. A body of another kind — a file upload, a GraphQL
// query — is not something the app can send, and it is left out rather than half-read.
func bodyText(from *body) string {
	if from == nil {
		return ""
	}
	if from.Mode == "raw" {
		return from.Raw
	}
	if from.Mode == "urlencoded" {
		parts := make([]string, 0, len(from.URLEncoded))
		for _, row := range from.URLEncoded {
			if row.Disabled {
				continue
			}
			parts = append(parts, row.Key+"="+row.Value)
		}
		return strings.Join(parts, "&")
	}
	return ""
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
// the list holds one — the design exports one from the context menu, and a file with one item is
// what Postman expects to be handed.
//
// Tokens stay tokens: a `{{name}}` is written as the text it is, and no value behind it is written
// anywhere. That is not a rule this function enforces — the app never gives it a value to write.
func Export(name string, items []domain.CollectionNode) ([]byte, error) {
	doc := document{
		Info: info{Name: name, Schema: Schema},
		Item: exported(items),
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("коллекция %q не записывается: %w", name, err)
	}
	return append(data, '\n'), nil
}

func exported(items []domain.CollectionNode) []item {
	out := make([]item, 0, len(items))
	for _, node := range items {
		entry := item{Name: node.Name}
		if node.Kind == domain.NodeFolder {
			entry.Item = exported(node.Items)
		} else {
			entry.Request = exportedRequest(node)
		}
		out = append(out, entry)
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
	if node.Body != "" {
		out.Body = &body{Mode: "raw", Raw: node.Body}
	}
	if node.Auth != nil {
		out.Auth = exportedAuth(*node.Auth)
	}
	return out
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
