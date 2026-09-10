package jsonapi

import (
	"bytes"
	"encoding/json"
)

// Key builds the canonical identifier for a resource: "type/id".
func Key(typ, id string) string { return typ + "/" + id }

// Index maps a resource key ("type/id") to the resource. It is built from both
// the top-level data (including compound documents whose data is an array) and
// the included resources.
type Index map[string]Resource

// BuildIndex constructs an Index for the document.
func BuildIndex(doc *Document) Index {
	idx := Index{}
	add := func(r Resource) {
		if r.Type != "" && r.ID != "" {
			idx[Key(r.Type, r.ID)] = r
		}
	}
	for _, r := range DataResources(doc) {
		add(r)
	}
	for _, r := range doc.Included {
		add(r)
	}
	return idx
}

// DataResources returns the resources held in the top-level data member, which
// may be a single resource, an array of resources, or null.
func DataResources(doc *Document) []Resource {
	raw := bytes.TrimSpace(doc.Data)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	switch raw[0] {
	case '{':
		var r Resource
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil
		}
		return []Resource{r}
	case '[':
		var rs []Resource
		if err := json.Unmarshal(raw, &rs); err != nil {
			return nil
		}
		return rs
	default:
		return nil
	}
}
