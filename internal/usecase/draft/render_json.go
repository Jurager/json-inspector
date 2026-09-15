package draft

// The JSON a rendered command carries: the init of a `fetch(url, {…})`, and the strings inside it.
//
// encoding/json writes the values; what it does not do is keep an object's keys in the order they
// were put in — it takes a map, and Go iterates a map in a different order every time — so the one
// thing written here is an object that remembers.

import (
	"encoding/json"
	"strings"
)

type jsonObject []jsonField

type jsonField struct {
	key   string
	value json.RawMessage
}

// Mirrors a plain JS object: a key already set keeps its position and takes the new value.
func (o *jsonObject) set(key string, value json.RawMessage) {
	for i := range *o {
		if (*o)[i].key == key {
			(*o)[i].value = value
			return
		}
	}
	*o = append(*o, jsonField{key: key, value: value})
}

func (o jsonObject) text() string {
	var b strings.Builder
	b.WriteByte('{')
	for i, f := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(jsonString(f.key))
		b.WriteByte(':')
		b.Write(f.value)
	}
	b.WriteByte('}')
	return b.String()
}

// jsonString writes a JSON string literal. Go's encoder, with HTML escaping off: `<`, `>` and `&`
// are ordinary characters in a body somebody wrote, and `<` in a command they copy out is noise
// that says nothing about the request.
func jsonString(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}
