package draft

// fetch, written as devtools hands it over: the URL quoted, then an init object. The method is
// always written, even a GET, because this is what somebody will paste into a console and read.

import (
	"encoding/json"

	"json-inspector/internal/domain"
)

func renderFetch(url, method string, headers []domain.HeaderPair, body string) string {
	var head jsonObject
	for _, h := range headers {
		head.set(h.Name, json.RawMessage(jsonString(h.Value)))
	}

	init := jsonObject{}
	init.set("method", json.RawMessage(jsonString(method)))
	if len(head) > 0 {
		init.set("headers", json.RawMessage(head.text()))
	}
	if body != "" {
		init.set("body", json.RawMessage(jsonString(body)))
	}
	return "fetch(" + shellQuote(url) + ", " + init.text() + ")"
}
