package command

import "strings"

// fromFetch reads what devtools' "Copy as fetch" writes. The init has to be strict JSON, so a JS
// object literal or a `JSON.stringify(…)` call cannot be read — the TS reports bad-fetch-init
// rather than guessing, and so does this.
func fromFetch(text string) (pendingRequest, Reason) {
	call := fetchCallRe.FindStringIndex(text)
	if call == nil {
		return pendingRequest{}, ReasonBadFetchInit
	}
	// The match ends at the opening parenthesis; the TS takes the same index from its own match.
	open := call[1] - 1
	closeIdx, ok := matchBracket(text, open, posixDialect)
	if !ok {
		return pendingRequest{}, ReasonBadFetchInit
	}

	args := splitTopLevel(text[open+1:closeIdx-1], ',', posixDialect)
	url, ok := jsString(args[0])
	if !ok {
		return pendingRequest{}, ReasonBadFetchInit
	}

	if len(args) < 2 || jsTrim(args[1]) == "" {
		return pendingRequest{method: "GET", url: url, entries: []headerEntry{}}, ""
	}

	init, ok := jsJSONParse(args[1])
	if !ok {
		// `fetch(url, options)` — an identifier we cannot resolve at all.
		return pendingRequest{}, ReasonBadFetchInit
	}

	entries := []headerEntry{}
	if headers, ok := init.get("headers"); ok {
		if headers.kind == jsArr {
			for _, pair := range headers.arr {
				if pair.kind == jsArr && len(pair.arr) >= 2 {
					entries = append(entries, headerEntry{
						name:  jsToString(pair.arr[0]),
						value: jsToString(pair.arr[1]),
					})
				}
			}
		} else if headers.kind == jsObj {
			// Object.entries, so the names come out in JS property order.
			for _, m := range headers.members() {
				value := ""
				if m.val.kind != jsNull {
					value = jsToString(m.val)
				}
				entries = append(entries, headerEntry{name: m.key, value: value})
			}
		}
	}

	body := ""
	if b, ok := init.get("body"); ok {
		switch b.kind {
		case jsStr:
			body = b.str
		case jsNum:
			body = jsNumberString(b.num)
		case jsBool:
			body = jsToString(b)
		case jsArr, jsObj:
			// A body written as an object or an array is sent as its JSON text.
			body = jsJSONStringify(b)
		}
	}

	method := "GET"
	if m, ok := init.get("method"); ok && m.kind == jsStr && m.str != "" {
		method = strings.ToUpper(m.str)
	}

	return pendingRequest{method: method, url: url, entries: entries, body: body}, ""
}
