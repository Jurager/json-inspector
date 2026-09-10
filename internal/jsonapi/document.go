// Package jsonapi implements parsing and analysis of JSON:API
// (https://jsonapi.org/) documents: detection, resource indexing and
// building the object graph ("карта объекта") used by the inspector UI.
package jsonapi

import "encoding/json"

// Document is a top-level JSON:API document.
type Document struct {
	Data     json.RawMessage `json:"data"`
	Errors   []ErrorObject   `json:"errors"`
	Meta     json.RawMessage `json:"meta"`
	Links    Links           `json:"links"`
	Included []Resource      `json:"included"`
	JSONAPI  json.RawMessage `json:"jsonapi"`
}

// ErrorObject is a JSON:API error object.
type ErrorObject struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Code   string          `json:"code"`
	Title  string          `json:"title"`
	Detail string          `json:"detail"`
	Source json.RawMessage `json:"source"`
	Meta   json.RawMessage `json:"meta"`
}

// Resource is a JSON:API resource object.
type Resource struct {
	Type          string                 `json:"type"`
	ID            string                 `json:"id"`
	Attributes    json.RawMessage        `json:"attributes"`
	Relationships map[string]Relationship `json:"relationships"`
	Links         Links                  `json:"links"`
	Meta          json.RawMessage        `json:"meta"`
}

// Relationship is a JSON:API relationship object.
type Relationship struct {
	Data  json.RawMessage `json:"data"`
	Links Links           `json:"links"`
	Meta  json.RawMessage `json:"meta"`
}

// ResourceIdentifier identifies a related resource.
type ResourceIdentifier struct {
	Type string          `json:"type"`
	ID   string          `json:"id"`
	Meta json.RawMessage `json:"meta"`
}

// Links maps a link name to a raw JSON value that is either a URL string or
// an object with an "href" member (per the JSON:API spec).
type Links map[string]json.RawMessage

// Href extracts the URL from the named link, accepting both the string form
// ("self": "http://…") and the object form ("self": {"href": "http://…"}).
func (l Links) Href(name string) string {
	raw, ok := l[name]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		Href string `json:"href"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Href
	}
	return ""
}
