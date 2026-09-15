// Package postman reads and writes the Postman v2.1 collection format: bytes in, a
// domain.Collection out, and back again. It is a pure package — no window, no database — so
// both directions can be tested against a file.
package postman

import "encoding/json"

// Schema is what a file written here says it is. Postman reads it, and so does anyone else looking
// at the JSON.
const Schema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

type collectionFile struct {
	Info info   `json:"info"`
	Auth *auth  `json:"auth,omitempty"`
	Item []item `json:"item"`
}

type info struct {
	Name   string `json:"name"`
	Schema string `json:"schema,omitempty"`
}

// item is a row of a collection file: a request is the thing that gets sent, a folder holds more.
// A folder is this app's "collection inside one" and keeps its own auth: a group that lost it on
// the way in would behave differently here.
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

// A path and not the bytes, which is what Postman keeps and what this app keeps: a collection file
// stays a description of the requests in it rather than a copy of everything they send.
type fileField struct {
	Src string `json:"src,omitempty"`
}

// auth is an authorization as Postman writes it: the scheme's name, and its fields under a key of
// that same name — `{"type":"bearer","bearer":[{"key":"token","value":"…"}]}`. The fields are a map
// and not a struct because they belong to the scheme, and two schemes differ from the rest.
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
