package command

import "strings"

// fromPowerShell reads an Invoke-RestMethod (or its iwr/irm aliases) command. PowerShell is the one
// tool whose flags are case-insensitive, which is why every lookup goes through ToLower.
func fromPowerShell(tokens []string) (pendingRequest, Reason) {
	var method optString
	url := ""
	body := ""
	entries := []headerEntry{}

	cursor := 0
	for cursor = 0; cursor < len(tokens); cursor++ {
		token := tokens[cursor]
		if !strings.HasPrefix(token, "-") {
			if url == "" && looksLikeURL(token) {
				url = token
				continue
			}
			// A second bare word is not part of a request line — `-InFile payload.json` lands here
			// because -InFile is not a flag this app can honour.
			return pendingRequest{}, ReasonLeftover
		}
		flag, inline := splitFlag(token)

		switch strings.ToLower(flag) {
		case "-method":
			if v := take(tokens, &cursor, inline); v.set {
				method = optString{value: strings.ToUpper(v.value), set: true}
			}
		case "-uri":
			// `take() ?? url`: a flag with no value leaves the URL as it was.
			if v := take(tokens, &cursor, inline); v.set {
				url = v.value
			}
		case "-body":
			v := take(tokens, &cursor, inline)
			if !v.set {
				return pendingRequest{}, ReasonUnsupportedVariable
			}
			body = v.value
		case "-headers":
			v := take(tokens, &cursor, inline)
			if !v.set || strings.HasPrefix(v.value, "$") {
				return pendingRequest{}, ReasonUnsupportedVariable
			}
			if strings.HasPrefix(v.value, "@{") {
				parsed, ok := psHashtable(v.value)
				if !ok {
					return pendingRequest{}, ReasonUnsupportedVariable
				}
				entries = append(entries, parsed...)
			}
		case "-contenttype":
			if v := take(tokens, &cursor, inline); v.set {
				entries = append(entries, headerEntry{name: "Content-Type", value: v.value})
			}
		default:
			if valueFlags[FormatPowerShell][flag] || valueFlags[FormatPowerShell][strings.ToLower(flag)] {
				// A flag whose value is a variable cannot be honoured, and silently dropping it
				// would send a different request than the one that was pasted.
				if v := take(tokens, &cursor, inline); v.set && strings.HasPrefix(v.value, "$") {
					return pendingRequest{}, ReasonUnsupportedVariable
				}
			}
		}
	}

	if url == "" {
		return pendingRequest{}, ReasonNoURL
	}

	return pendingRequest{
		method:  defaultMethod(method, body),
		url:     url,
		entries: withFormType(entries, body != ""),
		body:    body,
	}, ""
}

// psValue reads the right-hand side of a hashtable entry: a bare word, a quoted string, or a
// variable the port cannot resolve. The bool is false for the variable.
func psValue(raw string) (string, bool) {
	t := jsTrim(raw)
	if t == "" {
		return "", true
	}
	if strings.HasPrefix(t, "$") {
		return "", false
	}
	if strings.HasPrefix(t, "'") || strings.HasPrefix(t, `"`) {
		var r readResult
		var ok bool
		if t[0] == '\'' {
			r, ok = readSingle(t, 0, powershellDialect)
		} else {
			r, ok = readDouble(t, 0, powershellDialect)
		}
		if !ok {
			return "", false
		}
		return r.text, true
	}
	return t, true
}

// psHashtable reads a `@{ 'Name' = 'value'; … }` literal. An entry whose name or value is a
// variable, or which has no name at all, makes the whole table unreadable: half a header set is
// worse than none, because it looks like it worked.
func psHashtable(text string) ([]headerEntry, bool) {
	out := []headerEntry{}
	// text.slice(2, -1) drops the `@{` and the closing `}`. A literal that is only its opening — a
	// paste cut short, `-Headers '@{'` — has nothing between the two ends: the TS slice answers the
	// empty string there, and this has to as well rather than reading past the end of it.
	inner := ""
	if len(text) > 2 {
		inner = text[2 : len(text)-1]
	}
	for _, entry := range splitTopLevel(inner, ';', powershellDialect) {
		if jsTrim(entry) == "" {
			continue
		}
		parts := splitTopLevel(entry, '=', powershellDialect)
		name, nameOK := psValue(parts[0])
		value, valueOK := psValue(strings.Join(parts[1:], "="))
		if !nameOK || !valueOK || name == "" {
			return nil, false
		}
		out = append(out, headerEntry{name: name, value: value})
	}
	return out, true
}
