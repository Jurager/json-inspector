package draft

import (
	"context"
	"strings"

	"json-inspector/internal/domain"
)

// Preview is what the command line draws and cannot work out from the draft alone: which of its
// `{{tokens}}` mean nothing. Whether the request can go out follows from that and from a URL the
// window itself is holding, so there is no verdict here — only what the window cannot see for
// itself.
type Preview struct {
	Missing []string `json:"missing"`
}

// parts is every text of a draft a `{{token}}` can appear in, kept apart rather than flattened, so
// that filling the variables in and asking which are missing walk the same list in the same order.
type parts struct {
	url     string
	body    string
	headers []domain.HeaderPair
	cookie  string
	auth    string
}

func collect(d domain.Draft) parts {
	out := parts{url: d.URL, body: d.Body, auth: d.Auth.Token}
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

// texts lists the parts in the order a resolver answers them — a header contributes its name and
// then its value — which is the same order putBack reads the answers back in.
//
// The parameters are not here: they are in the URL, which is where they became rows from in the
// first place. The Auth token is here but is not put back: the chip has a slot in the design and
// no effect on a request yet.
func (p parts) texts() []string {
	out := make([]string, 0, 4+2*len(p.headers))
	out = append(out, p.url, p.body)
	for _, header := range p.headers {
		out = append(out, header.Name, header.Value)
	}
	return append(out, p.cookie, p.auth)
}

// putBack returns the parts with those answers in them. The headers are copied: the answers are a
// second rendering of the same texts, and filling one in must not disturb the other.
func (p parts) putBack(resolved []string) parts {
	at := 0
	p.url = resolved[at]
	at++
	p.body = resolved[at]
	at++

	headers := make([]domain.HeaderPair, len(p.headers))
	for i := range p.headers {
		headers[i] = domain.HeaderPair{Name: resolved[at], Value: resolved[at+1]}
		at += 2
	}
	p.headers = headers
	p.cookie = resolved[at]
	return p
}

func (u *UseCase) preview(ctx context.Context, draft domain.Draft) (Preview, error) {
	missing, err := u.vars.Missing(ctx, collect(draft).texts())
	if err != nil {
		return Preview{}, err
	}
	if missing == nil {
		missing = []string{}
	}
	return Preview{Missing: missing}, nil
}
