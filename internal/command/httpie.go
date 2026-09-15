package command

import (
	"strings"

	"json-inspector/internal/domain"
)

// fromHTTPie reads an httpie command. Its grammar is the odd one out: bare words are not operands
// but `name=value` items, and the separator inside them decides whether the item becomes a header,
// a query parameter or a field of the JSON body.
func fromHTTPie(tokens []string) (pendingRequest, Reason) {
	var method optString
	url := ""
	var raw optString
	entries := []headerEntry{}
	// The three flags that carry a credential, decided on once the loop is over: which scheme
	// `--auth` means depends on `--auth-type`, wherever in the line it was written.
	var authType optString
	var credential optString
	var bearer optString
	var fields []jsMember
	query := []string{}

	cursor := 0
	for cursor = 0; cursor < len(tokens); cursor++ {
		token := tokens[cursor]
		flag, inline := splitFlag(token)

		// Anything dash-led is a flag, including the `--flag=value` form.
		if strings.HasPrefix(token, "-") {
			switch flag {
			case "--raw":
				if v := take(tokens, &cursor, inline); v.set {
					raw = v
				}
			case "--auth", "-a":
				if v := take(tokens, &cursor, inline); v.set && v.value != "" {
					credential = v
				}
			case "--auth-type":
				if v := take(tokens, &cursor, inline); v.set {
					authType = optString{value: strings.ToLower(v.value), set: true}
				}
			case "--bearer":
				if v := take(tokens, &cursor, inline); v.set && v.value != "" {
					bearer = v
				}
			default:
				if valueFlags[FormatHTTPie][flag] {
					take(tokens, &cursor, inline)
				}
			}
			continue
		}

		if url == "" && isURLOperand(token) {
			url = token
			continue
		}
		if !method.set && methods[strings.ToUpper(token)] {
			method = optString{value: strings.ToUpper(token), set: true}
			continue
		}

		op, at, ok := httpieOperator(token)
		if !ok || at == 0 {
			return pendingRequest{}, ReasonLeftover
		}
		name := token[:at]
		value := token[at+len(op):]

		switch op {
		case ":=", "@":
			// A typed or file field has no representation this app can send.
			return pendingRequest{}, ReasonUnsupportedField
		case "==":
			query = append(query, name+"="+jsEncodeURIComponent(value))
		case ":":
			entries = append(entries, headerEntry{name: name, value: value})
		default:
			fields = append(fields, jsMember{key: name, val: jsValue{kind: jsStr, str: value}})
		}
	}

	if url == "" {
		return pendingRequest{}, ReasonNoURL
	}

	body := ""
	if raw.set {
		body = raw.value
	}
	if body == "" && len(fields) > 0 {
		obj := jsValue{kind: jsObj}
		for _, f := range fields {
			// Object.fromEntries: a repeated name keeps its first position and takes the last value.
			obj.set(f.key, f.val)
		}
		body = jsJSONStringify(obj)
		if !hasHeader(entries, "content-type") {
			entries = append(entries, headerEntry{name: "Content-Type", value: "application/json"})
		}
	}

	// httpie names the scheme separately from the credential, and `digest` is the only other one this
	// app has: an `--auth-type` it does not know is read as Basic, which is what httpie itself does
	// when it is not told otherwise.
	auth := (*domain.Auth)(nil)
	if bearer.set {
		auth = bearerScheme(bearer.value)
	} else if credential.set {
		login, password := splitCredential(credential.value)
		if authType.set && authType.value == "digest" {
			auth = digestScheme(login, password)
		} else {
			auth = basicScheme(login, password)
		}
	}

	result := pendingRequest{
		method:  defaultMethod(method, body),
		url:     url,
		entries: entries,
		body:    body,
		auth:    auth,
	}
	if len(query) > 0 {
		result.url = appendQuery(url, strings.Join(query, "&"))
	}
	return result, ""
}

// httpieOperator finds the separator that turns a bare word into an item: `:=` and `==` first,
// because they are two characters, then the single `=`, `:` and `@`.
func httpieOperator(token string) (string, int, bool) {
	for i := 0; i < len(token); i++ {
		last := i + 2
		if last > len(token) {
			last = len(token)
		}
		if two := token[i:last]; two == ":=" || two == "==" {
			return two, i, true
		}
		switch token[i] {
		case '=', ':', '@':
			return token[i : i+1], i, true
		}
	}
	return "", 0, false
}

// hasHeader reports whether a header of this name was already collected, whatever its casing.
func hasHeader(entries []headerEntry, name string) bool {
	for _, e := range entries {
		if strings.ToLower(e.name) == name {
			return true
		}
	}
	return false
}
