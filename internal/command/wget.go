package command

import "strings"

// fromWget reads a wget command. Unlike curl it takes no `--`, and its body flags overwrite each
// other rather than accumulating: the last one wins.
func fromWget(tokens []string) (pendingRequest, Reason) {
	var method optString
	body := ""
	entries := []headerEntry{}
	positionals := []string{}

	cursor := 0
	for cursor = 0; cursor < len(tokens); cursor++ {
		raw := tokens[cursor]
		flag, inline := splitFlag(raw)
		if inline == nil && !strings.HasPrefix(raw, "-") {
			positionals = append(positionals, raw)
			continue
		}

		switch flag {
		case "--method":
			if v := take(tokens, &cursor, inline); v.set {
				method = optString{value: strings.ToUpper(v.value), set: true}
			}
		case "--header":
			// An empty argument is not a header; anything else is one even without a colon.
			if v := take(tokens, &cursor, inline); v.set && v.value != "" {
				entries = append(entries, parseHeaderArg(v.value))
			}
		case "--body-data", "--post-data":
			// `take() ?? ''` — a bare flag at the end clears whatever was collected before it.
			if v := take(tokens, &cursor, inline); v.set {
				body = v.value
			} else {
				body = ""
			}
		default:
			if valueFlags[FormatWget][flag] {
				take(tokens, &cursor, inline)
			}
		}
	}

	url := pickURL(positionals)
	if url == "" {
		return pendingRequest{}, ReasonNoURL
	}
	if hasLeftover(positionals, url) {
		return pendingRequest{}, ReasonLeftover
	}

	return pendingRequest{
		method:  defaultMethod(method, body),
		url:     url,
		entries: withFormType(entries, body != ""),
		body:    body,
	}, ""
}
