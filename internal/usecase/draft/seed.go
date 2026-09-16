package draft

import (
	"context"
	"strings"

	"json-inspector/internal/domain"
)

// Replace hands the draft a whole request. Everything the draft held is dropped, including the
// parameters the old URL carried and the choice made in the Auth chip: a seed is a request, not an
// edit to one.
func (u *UseCase) Replace(ctx context.Context, id domain.DraftID, seed Seed) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		d.Method = seed.Method
		d.URL = seed.URL
		d.Body = seed.Body
		d.BodyKind = domain.KindOf(seed.BodyKind)
		d.Form = u.formWithIDs(seed.Form)
		d.BodyFile = seed.BodyFile
		d.Auth = authOf(seed)
		d.Params = u.rowsFromURL(seed.URL, nil)
		d.Headers = u.rowsFromHeaders(seed.Headers)
		d.Cookies = u.cookiesFromSeed(seed)
		return nil
	})
}

// formWithIDs is withRowIDs for a form body: a row the window cannot address is a row it cannot
// edit.
func (u *UseCase) formWithIDs(rows []domain.FormRow) []domain.FormRow {
	out := make([]domain.FormRow, 0, len(rows))
	for _, row := range rows {
		if row.ID == "" {
			row.ID = u.ids()
		}
		out = append(out, row)
	}
	return out
}

// A seed that says nothing about authorization is not one that forgot, it is one with none. It is
// normalized like one the window sent: a seed comes from outside, and a scheme that arrived
// without its starting answers would be drawn with a blank where its first choice belongs.
func authOf(seed Seed) domain.Auth {
	if seed.Auth == nil {
		return domain.NewAuth(domain.AuthNone)
	}
	return seed.Auth.Normalized()
}

// Prepared is the draft the window is editing, ready to go out. inherits is what its «Inherit»
// resolves to, and the caller is the one that knows: the draft holds what was typed, and where in a
// tree the request sits is not part of that. It is nil for a draft with nothing above it, which is
// the command line's.
func (u *UseCase) Prepared(
	ctx context.Context,
	id domain.DraftID,
	inherits *domain.Auth,
) (Prepared, error) {
	draft, err := u.Current(ctx, id)
	if err != nil {
		return Prepared{}, err
	}
	return u.prepare(ctx, draft, inherits)
}

// Prepare fills in a request that is not the one being composed — a followed link, a collection run
// — without disturbing any draft. Resolving and masking live here and not with the caller, so there
// is one answer to what a request looks like when it leaves.
func (u *UseCase) Prepare(ctx context.Context, seed Seed) (Prepared, error) {
	draft := domain.Draft{
		Method:   seed.Method,
		URL:      seed.URL,
		Body:     seed.Body,
		BodyKind: domain.KindOf(seed.BodyKind),
		Form:     u.formWithIDs(seed.Form),
		BodyFile: seed.BodyFile,
		Headers:  u.rowsFromHeaders(seed.Headers),
		Cookies:  seed.Cookies,
		Auth:     authOf(seed),
	}
	// A request with no jar of its own — a followed link, a pasted command — carries its cookies in
	// the header, and that is where they are read from.
	if len(draft.Cookies) == 0 {
		draft.Cookies = cookiesFromHeaders(seed.Headers)
	}
	return u.prepare(ctx, draft, nil)
}

// withRowIDs gives every row an id. A draft that came from somewhere else — a saved request — has
// rows without them, and the window addresses a row by id: one that has none could not be edited.
func (u *UseCase) withRowIDs(d domain.Draft) domain.Draft {
	for i := range d.Params {
		if d.Params[i].ID == "" {
			d.Params[i].ID = u.ids()
		}
	}
	for i := range d.Headers {
		if d.Headers[i].ID == "" {
			d.Headers[i].ID = u.ids()
		}
	}
	for i := range d.Cookies {
		if d.Cookies[i].ID == "" {
			d.Cookies[i].ID = u.ids()
		}
		if d.Cookies[i].Path == "" {
			d.Cookies[i].Path = "/"
		}
	}
	for i := range d.Form {
		if d.Form[i].ID == "" {
			d.Form[i].ID = u.ids()
		}
	}
	return d
}

// rowsFromHeaders turns the headers of a seed into rows. A `Cookie` header is kept: the jar is
// derived from it, and it is the collect step — not this one — that decides which of the two goes
// out when a seed brought both.
func (u *UseCase) rowsFromHeaders(headers []domain.HeaderPair) []domain.Row {
	rows := []domain.Row{}
	for _, header := range headers {
		rows = append(rows,
			domain.Row{ID: u.ids(), Name: header.Name, Value: header.Value, Enabled: true})
	}
	return rows
}

// cookiesFromSeed is a seed's jar as rows. A seed that came without one — a pasted command, a
// followed link — carries its cookies in a `Cookie` header, and that is where they are read from.
func (u *UseCase) cookiesFromSeed(seed Seed) []domain.CookieRow {
	rows := seed.Cookies
	if len(rows) == 0 {
		rows = cookiesFromHeaders(seed.Headers)
	}

	out := make([]domain.CookieRow, 0, len(rows))
	for _, row := range rows {
		row.ID = u.ids()
		if row.Path == "" {
			row.Path = "/"
		}
		out = append(out, row)
	}
	return out
}

func cookiesFromHeaders(headers []domain.HeaderPair) []domain.CookieRow {
	for _, header := range headers {
		if strings.EqualFold(header.Name, "Cookie") {
			return cookiesFromHeader(header.Value)
		}
	}
	return nil
}
