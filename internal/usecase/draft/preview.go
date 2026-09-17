package draft

import (
	"context"
	"strings"

	"json-inspector/internal/domain"
)

// Preview is what the command line cannot work out from the draft alone: which of its `{{tokens}}`
// mean nothing. There is no verdict here — the rest follows from this and a URL the window holds.
type Preview struct {
	Missing []string `json:"missing"`
}

// parts is every text of a draft a `{{token}}` can appear in, kept apart rather than flattened, so
// that filling the variables in and asking which are missing walk the same list in the same order.
type parts struct {
	url     string
	body    string
	form    []domain.FormRow
	headers []domain.HeaderPair
	cookie  string
	auth    domain.Auth
}

func collect(d domain.Draft) parts {
	out := parts{url: d.URL, body: d.Body, form: d.Form, auth: d.Auth}
	// The jar owns the Cookie header: it is what the "Cookies" tab edits, and a header row with the
	// same name would be the same cookies a second time.
	out.cookie = headerFromCookies(d.Cookies)

	for _, row := range d.Headers {
		if !row.Enabled || strings.TrimSpace(row.Name) == "" {
			continue
		}
		if out.cookie != "" && strings.EqualFold(row.Name, "Cookie") {
			continue
		}
		out.headers = append(out.headers, domain.HeaderPair{Name: row.Name, Value: row.Value})
	}
	return out
}

// The parts in the order a resolver answers them, which is the order putBack reads them back in.
// Parameters are not here: they live in the URL. Auth fields go in scheme order, not map order —
// which is random in Go — because texts and putBack have to agree on the list. The answers to the
// authorization's fields are here last, and they are put back: one of them is what becomes the
// Authorization header, and a `{{token}}` in any of them has to be filled in like any other text.
func (p parts) texts() []string {
	keys := domain.FieldKeys(p.auth.Type)
	out := make([]string, 0, 4+3*len(p.form)+2*len(p.headers)+len(keys))
	out = append(out, p.url, p.body)
	// A form row contributes its name, its value and the path it may carry: a `{{token}}` reaches a
	// form body the same way it reaches a header, and a row it could not reach would go out literal.
	for _, row := range p.form {
		out = append(out, row.Name, row.Value, row.Src)
	}
	for _, header := range p.headers {
		out = append(out, header.Name, header.Value)
	}
	out = append(out, p.cookie)
	for _, key := range keys {
		out = append(out, p.auth.Answer(key))
	}
	return out
}

// putBack returns the parts with those answers in them. The headers are copied: the answers are a
// second rendering of the same texts, and filling one in must not disturb the other.
func (p parts) putBack(resolved []string) parts {
	at := 0
	p.url = resolved[at]
	at++
	p.body = resolved[at]
	at++

	form := make([]domain.FormRow, len(p.form))
	for i := range p.form {
		form[i] = p.form[i]
		form[i].Name, form[i].Value, form[i].Src = resolved[at], resolved[at+1], resolved[at+2]
		at += 3
	}
	p.form = form

	headers := make([]domain.HeaderPair, len(p.headers))
	for i := range p.headers {
		headers[i] = domain.HeaderPair{Name: resolved[at], Value: resolved[at+1]}
		at += 2
	}
	p.headers = headers
	p.cookie = resolved[at]
	at++

	fields := make(map[string]string, len(p.auth.Fields))
	for _, key := range domain.FieldKeys(p.auth.Type) {
		fields[key] = resolved[at]
		at++
	}
	p.auth = domain.Auth{Type: p.auth.Type, Fields: fields}
	return p
}

func (u *UseCase) preview(ctx context.Context, draft domain.Draft) (Preview, error) {
	missing, err := u.vars.Missing(ctx, nil, collect(draft).texts())
	if err != nil {
		return Preview{}, err
	}
	if missing == nil {
		missing = []string{}
	}
	return Preview{Missing: missing}, nil
}
