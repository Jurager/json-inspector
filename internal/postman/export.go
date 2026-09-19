package postman

import (
	"encoding/json"
	"fmt"
	"strings"

	"json-inspector/internal/domain"
)

// Export writes a collection as a file: the whole collection, or the one request the context
// menu exports — one item is what Postman expects to be handed. A `{{name}}` in an address or a
// body stays text, and what a level answers for the names it owns travels beside them: a collection
// that lost its own values would mean something else on the other side, which is what they are for.
//
// Nothing here has to be kept back for being secret. A collection cannot hold one — SaveVariables
// refuses it for exactly this reason — so every value written is one the user meant to hand over.
func Export(collection domain.Collection) ([]byte, error) {
	doc := collectionFile{
		Info:     info{Name: collection.Name, Schema: Schema},
		Variable: exportedVariables(collection.Variables),
		Item:     exportLevel(collection),
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

// exportLevel writes a level the way the file keeps it: the requests and the collections inside
// them in one list, in the order the tree draws them.
func exportLevel(collection domain.Collection) []item {
	out := make([]item, 0, len(collection.Items)+len(collection.Children))
	for _, entry := range collection.Level() {
		if entry.Collection != nil {
			group := item{
				Name:     entry.Collection.Name,
				Variable: exportedVariables(entry.Collection.Variables),
			}
			if entry.Collection.Auth != nil {
				group.Auth = exportedAuth(*entry.Collection.Auth)
			}
			group.Item = exportLevel(*entry.Collection)
			out = append(out, group)
			continue
		}
		out = append(out, item{Name: entry.Node.Name, Request: exportedRequest(*entry.Node)})
	}
	return out
}

// exportedVariables writes what a level answers for `{{tokens}}`. A row with no name is not a
// variable — it is a row somebody left empty, and a name is the only thing that makes one.
func exportedVariables(variables []domain.Variable) []variable {
	out := make([]variable, 0, len(variables))
	for _, v := range variables {
		if v.Name == "" {
			continue
		}
		out = append(out, variable{
			Key:      v.Name,
			Value:    v.Value,
			Type:     "string",
			Disabled: !v.Enabled,
		})
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

// json, xml and raw all leave as `raw`: Postman has nowhere to record which of the three a body
// was, and an `options.raw.language` this reader does not honour would be a promise the next
// import breaks. A form and a file survive the trip — Postman has a shape for each.
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

// Every field the scheme declares is written, empty or not: other people's tools read the file,
// and a missing field reads as one that does not exist rather than one nobody filled in. The
// order is the scheme's own, which is the order Postman draws the fields in.
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
