package draft

import (
	"encoding/base64"
	"strings"

	"json-inspector/internal/domain"
)

func fromCurl(tokens []string) (pendingRequest, CommandReason) {
	var method optString
	var explicitURL optString
	var user optString
	nonBasicAuth := false
	digest := false
	bearer := optString{}
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
		case "--digest":
			digest = true
		case "--oauth2-bearer":
			if v := take(tokens, &cursor, inline); v.set {
				bearer = v
			}
		case "--ntlm", "--negotiate", "--anyauth", "--proxy-anyauth":
			// Schemes this app has no answer for. Inventing a Basic credential for them would be
			// sending one the command never asked for, so `-u` is left alone.
			nonBasicAuth = true
		default:
			if curlFlags[flag] {
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

	// A token the command handed over outright wins over a login it also carries: curl takes both, and
	// the one the server reads is the bearer one.
	auth := (*domain.Auth)(nil)
	if bearer.set {
		auth = bearerScheme(bearer.value)
	} else if user.set && !nonBasicAuth {
		login, password := splitCredential(user.value)
		if digest {
			auth = digestScheme(login, password)
		} else {
			auth = basicScheme(login, password)
		}
	}

	// A header is what every tool actually writes — devtools copies `-H 'Authorization: …'` and never
	// `-u` — and left as a header the chip would never learn what the request authorizes itself with.
	// It wins over a flag, the precedence a written header has always had over a scheme.
	entries, auth = writtenAuth(entries, auth)

	body := strings.Join(data, "&")
	if body == "" && len(forms) > 0 {
		body = joinForm(forms)
	}

	result := pendingRequest{
		url:     url,
		entries: withFormType(entries, len(data) > 0 || len(forms) > 0),
		body:    body,
		auth:    auth,
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

// The row goes with the credential: left beside the scheme it would win over it, since a written
// header is what goes out, and editing the chip would then do nothing. A scheme that cannot be
// rebuilt from a header — Digest, Negotiate, NTLM — stays the header it is: Digest holds no
// password to put in a field, and Negotiate and NTLM have no fields at all.
func writtenAuth(entries []headerEntry, auth *domain.Auth) ([]headerEntry, *domain.Auth) {
	for i, e := range entries {
		if !strings.EqualFold(e.name, "Authorization") {
			continue
		}
		scheme, ok := authFromHeader(e.value)
		if !ok {
			return entries, auth
		}
		rest := make([]headerEntry, 0, len(entries)-1)
		rest = append(rest, entries[:i]...)
		return append(rest, entries[i+1:]...), scheme
	}
	return entries, auth
}

func authFromHeader(value string) (*domain.Auth, bool) {
	name, rest, found := strings.Cut(strings.TrimSpace(value), " ")
	if !found {
		return nil, false
	}
	switch {
	case strings.EqualFold(name, "basic"):
		raw, err := decodeBase64(strings.TrimSpace(rest))
		if err != nil {
			return nil, false
		}
		login, password, ok := strings.Cut(raw, ":")
		if !ok {
			return nil, false
		}
		return basicScheme(login, password), true
	case strings.EqualFold(name, "bearer"):
		if token := strings.TrimSpace(rest); token != "" {
			return bearerScheme(token), true
		}
	}
	return nil, false
}

// A Basic credential's padding is optional in practice — both spellings are written by clients in
// the wild — so both are tried.
func decodeBase64(s string) (string, error) {
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(raw), nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func joinForm(forms []headerEntry) string {
	parts := make([]string, 0, len(forms))
	for _, f := range forms {
		parts = append(parts, f.name+"="+f.value)
	}
	return strings.Join(parts, "&")
}
