package command

import "strings"

// fromCurl reads a curl command. It is the richest of the five: curl needs the most flags before
// the URL, and it is the one that can turn data into a query string, a form body or an upload.
func fromCurl(tokens []string) (pendingRequest, Reason) {
	var method optString
	var explicitURL optString
	var user optString
	nonBasicAuth := false
	get := false
	entries := []headerEntry{}
	data := []string{}
	forms := []headerEntry{}
	positionals := []string{}

	cursor := 0
	for cursor = 0; cursor < len(tokens); cursor++ {
		raw := tokens[cursor]
		if raw == "--" {
			// Everything after `--` is an operand, even if it starts with a dash.
			positionals = append(positionals, tokens[cursor+1:]...)
			break
		}

		flag, inline := splitFlag(raw)
		if inline == nil && !strings.HasPrefix(raw, "-") {
			positionals = append(positionals, raw)
			continue
		}

		switch flag {
		case "-X", "--request":
			if v := take(tokens, &cursor, inline); v.set {
				method = optString{value: strings.ToUpper(v.value), set: true}
			}
		case "-H", "--header":
			// An empty argument is not a header; anything else is one even without a colon.
			if v := take(tokens, &cursor, inline); v.set && v.value != "" {
				entries = append(entries, parseHeaderArg(v.value))
			}
		case "-d", "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode":
			if v := take(tokens, &cursor, inline); v.set {
				data = append(data, v.value)
			}
		case "--json":
			if v := take(tokens, &cursor, inline); v.set {
				data = append(data, v.value)
				entries = append(entries, headerEntry{name: "Content-Type", value: "application/json"})
			}
		case "--url":
			if v := take(tokens, &cursor, inline); v.set {
				explicitURL = v
			}
		case "-I", "--head":
			method = optString{value: "HEAD", set: true}
		case "-G", "--get":
			get = true
		case "-u", "--user":
			if v := take(tokens, &cursor, inline); v.set {
				user = v
			}
		case "-A", "--user-agent":
			if v := take(tokens, &cursor, inline); v.set {
				entries = append(entries, headerEntry{name: "User-Agent", value: v.value})
			}
		case "-b", "--cookie":
			// A value without `=` names a cookie *file*, not a cookie.
			if v := take(tokens, &cursor, inline); v.set && v.value != "" && strings.Contains(v.value, "=") {
				entries = append(entries, headerEntry{name: "Cookie", value: v.value})
			}
		case "-F", "--form", "--form-string":
			if v := take(tokens, &cursor, inline); v.set {
				eq := strings.Index(v.value, "=")
				name, value := v.value, ""
				if eq != -1 {
					name, value = v.value[:eq], v.value[eq+1:]
				}
				// `-F 'file=@path'` is an upload this app has no way to reproduce.
				if strings.HasPrefix(value, "@") || strings.HasPrefix(value, "<") {
					return pendingRequest{}, ReasonUnsupportedMultipart
				}
				forms = append(forms, headerEntry{name: name, value: value})
			}
		case "--digest", "--ntlm", "--negotiate", "--anyauth", "--proxy-anyauth":
			// Any of these means the server is not asked for basic auth, so `-u` is left alone.
			nonBasicAuth = true
		default:
			if valueFlags[FormatCurl][flag] {
				take(tokens, &cursor, inline)
			}
		}
	}

	url := pickURL(positionals)
	if explicitURL.set {
		url = explicitURL.value
	}
	if url == "" {
		return pendingRequest{}, ReasonNoURL
	}
	if hasLeftover(positionals, url) {
		return pendingRequest{}, ReasonLeftover
	}

	if user.set && !nonBasicAuth {
		entries = append(entries, basicAuthEntry(user.value))
	}

	body := strings.Join(data, "&")
	if body == "" && len(forms) > 0 {
		body = joinForm(forms)
	}

	result := pendingRequest{
		url:     url,
		entries: withFormType(entries, len(data) > 0 || len(forms) > 0),
		body:    body,
	}
	switch {
	case method.set:
		result.method = method.value
	case body != "" && !get:
		result.method = "POST"
	default:
		result.method = "GET"
	}
	if get {
		// `-G` moves the data into the query string instead of sending it.
		result.url = appendQuery(result.url, body)
		result.body = ""
	}
	return result, ""
}

// joinForm writes the -F pairs as one urlencoded body.
func joinForm(forms []headerEntry) string {
	parts := make([]string, 0, len(forms))
	for _, f := range forms {
		parts = append(parts, f.name+"="+f.value)
	}
	return strings.Join(parts, "&")
}
