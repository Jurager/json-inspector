package postman

import (
	"encoding/json"
	"fmt"
	"strings"

	"json-inspector/internal/domain"
)

// Writing a collection: this app's tree becomes a Postman item tree.

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

// exported writes a level the way the file keeps it: the requests and the collections inside
// them in one list, in the order the tree draws them.
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
		out.Header = append(out.Header,
			field{Key: header.Name, Value: header.Value, Disabled: !header.Enabled})
	}
	// The parameters are written beside the address and not only inside it: a switched-off row is
	// not in the address, and this is the only place it can travel.
	for _, param := range node.Params {
		if strings.TrimSpace(param.Name) == "" {
			continue
		}
		out.URL.Query = append(out.URL.Query,
			field{Key: param.Name, Value: param.Value, Disabled: !param.Enabled})
	}
	out.Body = exportedBody(node)
	if node.Auth != nil {
		out.Auth = exportedAuth(*node.Auth)
	}
	return out
}

// exportedBody is the body as the file keeps it, in the mode that describes it.
//
// json, xml and raw all leave as `raw`: Postman has nowhere to record which of the three a
// body was, and writing an `options.raw.language` this reader does not honour would be a promise
// the next import would break. A form and a file do survive the trip, because Postman has a shape
// for each.
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
	written, ok := schemes[from.Type]
	if !ok {
		return nil
	}
	out := &auth{Type: written.Name, Extra: map[string][]field{}}
	fields := []field{}
	for _, key := range domain.FieldKeys(from.Type) {
		name, ok := written.Fields[key]
		if !ok {
			continue
		}
		fields = append(fields, field{Key: name, Value: from.Answer(key)})
	}
	out.Extra[written.Name] = fields
	return out
}
