package jsonapi

import (
	"bytes"
	"encoding/json"
)

// Parse decodes raw into a Document. It does not validate conformance beyond
// what encoding/json enforces.
func Parse(raw []byte) (*Document, error) {
	var d Document
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// IsJSONAPI reports whether raw is likely a JSON:API document. A document is
// treated as JSON:API when it has a top-level "jsonapi" member, an "included"
// member, or a "data" member whose value is a resource (or array of resources)
// each carrying both "type" and "id" string members.
func IsJSONAPI(raw []byte) bool {
	var probe struct {
		JSONAPI  json.RawMessage `json:"jsonapi"`
		Data     json.RawMessage `json:"data"`
		Included json.RawMessage `json:"included"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	if len(probe.JSONAPI) > 0 {
		return true
	}
	if len(probe.Included) > 0 {
		return true
	}
	if len(probe.Data) > 0 {
		return resourcesHaveTypeID(probe.Data)
	}
	return false
}

// resourcesHaveTypeID reports whether a raw "data" value is a resource object
// or a non-empty array of resource objects, all with string "type" and "id".
func resourcesHaveTypeID(data json.RawMessage) bool {
	d := bytes.TrimSpace(data)
	if len(d) == 0 || bytes.Equal(d, []byte("null")) {
		return false
	}
	switch d[0] {
	case '{':
		var r Resource
		if json.Unmarshal(d, &r) != nil {
			return false
		}
		return r.Type != "" && r.ID != ""
	case '[':
		var rs []Resource
		if json.Unmarshal(d, &rs) != nil {
			return false
		}
		if len(rs) == 0 {
			return false
		}
		for _, r := range rs {
			if r.Type == "" || r.ID == "" {
				return false
			}
		}
		return true
	default:
		return false
	}
}
