package postman

import (
	"encoding/json"
	"strings"

	"json-inspector/internal/domain"
)

// Ids are left empty: minting them is the tree's business, not this package's. A file that is not
// a collection is refused rather than guessed at — a half-imported collection is worse than a
// message saying what went wrong.
func Import(data []byte) (domain.Collection, error) {
	var doc collectionFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return domain.Collection{}, domain.Refuse(domain.CodeNotJSON, domain.ErrNotAllowed, nil)
	}
	if doc.Info.Name == "" && len(doc.Item) == 0 {
		return domain.Collection{}, domain.Refuse(domain.CodeNotPostman, domain.ErrNotAllowed, nil)
	}

	imported := domain.Collection{
		Name:        doc.Info.Name,
		Description: "",
		Variables:   importedVariables(doc.Variable),
	}
	if doc.Auth != nil {
		imported.Auth = authOf(*doc.Auth)
	}
	readLevel(doc.Item, &imported)
	return imported, nil
}

// importedVariables reads what a level answers for `{{tokens}}`, in the order the file wrote them:
// the order is the one the editor draws and the nearest answer is the last, so it is not a thing to
// sort. A row with no name is a row somebody left empty rather than a variable.
func importedVariables(from []variable) []domain.Variable {
	out := []domain.Variable{}
	for _, v := range from {
		name := strings.TrimSpace(v.Key)
		if name == "" {
			continue
		}
		out = append(out, domain.Variable{
			Name:     name,
			Value:    variableValue(v.Value),
			Kind:     domain.VariableText,
			Enabled:  !v.Disabled,
			Position: len(out) + 1,
		})
	}
	return out
}

// variableValue is a value out of a file as the text a request substitutes. A string is the usual
// case; the format allows any JSON, and a number or a flag becomes its own spelling rather than
// being dropped — `2` and `"2"` answer `{{page}}` the same way.
func variableValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// The order the rows are written in is the order the tree draws and a run walks them, and the two
// kinds share one position line numbered by where the row stood in the file — so a level merges
// back into the file's order rather than into two groups.
func readLevel(items []item, into *domain.Collection) {
	into.Items = []domain.CollectionNode{}
	into.Children = []domain.Collection{}

	for i, entry := range items {
		position := int64(i)
		if entry.Request != nil {
			into.Items = append(into.Items, requestNode(entry.Name, position, *entry.Request))
			continue
		}

		nested := domain.Collection{
			Name:      entry.Name,
			Position:  position,
			Variables: importedVariables(entry.Variable),
		}
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

// A scheme this app does not have is left out rather than guessed at: an unknown authorization is
// nothing here, and a level with no auth of its own is one that inherits.
func authOf(from auth) *domain.Auth {
	kind, written, ok := schemeOf(from.Type)
	if !ok {
		return nil
	}
	fields := map[string]string{}
	for key, name := range written.Fields {
		fields[key] = fieldValue(from.Extra[written.Name], name)
	}
	return &domain.Auth{Type: kind, Fields: fields}
}

func fieldValue(rows []field, key string) string {
	for _, row := range rows {
		if row.Key == key {
			return row.Value
		}
	}
	return ""
}
