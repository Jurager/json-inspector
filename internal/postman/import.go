package postman

import (
	"encoding/json"
	"strings"

	"json-inspector/internal/domain"
)

// Reading a collection: a Postman item tree becomes this app's, a request at a time.

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
	readLevel(doc.Item, &imported)
	return imported, nil
}

// level reads one level of a file into the collection that holds it, keeping the order the rows are
// written in: that order is the order the tree draws them and the order a run walks them, and it is
// one list for requests and folders together.
//
// A row with a request is a request; a row with items and no request is a folder, which in this app
// is a collection inside one — a group with a name, its own authorization and its own place. The
// two kinds share one position line here, numbered by where the row stood in the file, so that the
// level merges back into the file's order on the way out. readLevel turns one level of the file's
// item tree into one of this app's, in place.
func readLevel(items []item, into *domain.Collection) {
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
		readLevel(entry.Item, &nested)
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
// `raw` is not guessed into json or xml: the file does not say which it is, and guessing would
// change what a request that was saved elsewhere sends here. A GraphQL body is still left out
// rather than half-read — it is not a body this app can compose.
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
	kind, written, ok := typeOf(from.Type)
	if !ok {
		return nil
	}
	fields := map[string]string{}
	for key, name := range written.Fields {
		fields[key] = fieldValue(from.Extra[written.Name], name)
	}
	return &domain.Auth{Type: kind, Fields: fields}
}

// fieldValue is what one of the file's fields holds, by the name the format gives it.
func fieldValue(rows []field, key string) string {
	for _, row := range rows {
		if row.Key == key {
			return row.Value
		}
	}
	return ""
}
