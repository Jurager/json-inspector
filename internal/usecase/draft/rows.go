package draft

// The three row lists — parameters, headers and cookies, and the form rows of a form body —
// edited a row at a time. A row is addressed by its id, never by its index.

import (
	"context"

	"json-inspector/internal/domain"
)

// AddRow appends an empty row to one of the three lists. Cookies are not rows: the jar carries the
// attributes a Set-Cookie reply has, and none of them belong to a request.
func (u *UseCase) AddRow(
	ctx context.Context,
	id domain.DraftID,
	kind domain.RowKind,
) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		switch kind {
		case domain.RowParams:
			d.Params = append(d.Params, domain.Row{ID: u.ids(), Enabled: true})
		case domain.RowHeaders:
			d.Headers = append(d.Headers, domain.Row{ID: u.ids(), Enabled: true})
		case domain.RowCookies:
			d.Cookies = append(d.Cookies, domain.CookieRow{ID: u.ids(), Path: "/"})
		case domain.RowForm:
			d.Form = append(d.Form, domain.FormRow{ID: u.ids(), Enabled: true})
		default:
			return unknownKind(kind)
		}
		return nil
	})
}

// RemoveRow drops a row by id. A row that is not there is not an error: a double click, or a patch
// that arrives after its row is gone, has asked for exactly what it got.
//
// The row is rowID and the draft is id, because a row id and a draft id are different things in the
// same call: every other method on this use case names the draft the way this one does.
func (u *UseCase) RemoveRow(
	ctx context.Context,
	id domain.DraftID,
	kind domain.RowKind,
	rowID string,
) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		switch kind {
		case domain.RowParams:
			d.Params = without(d.Params, rowID)
			syncURL(d)
		case domain.RowHeaders:
			d.Headers = without(d.Headers, rowID)
		case domain.RowCookies:
			d.Cookies = withoutCookie(d.Cookies, rowID)
		case domain.RowForm:
			d.Form = withoutForm(d.Form, rowID)
		default:
			return unknownKind(kind)
		}
		return nil
	})
}

// PatchRow changes the fields a patch names and leaves the rest. A parameter edit writes the list
// back into the URL, because the URL is what goes out and the rows are only how it is edited.
func (u *UseCase) PatchRow(
	ctx context.Context,
	id domain.DraftID,
	kind domain.RowKind,
	rowID string,
	patch RowPatch,
) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		switch kind {
		case domain.RowParams:
			at := findRow(d.Params, rowID)
			if at < 0 {
				return notFound(kind, rowID)
			}
			applyPatch(&d.Params[at], patch)
			syncURL(d)
		case domain.RowHeaders:
			at := findRow(d.Headers, rowID)
			if at < 0 {
				return notFound(kind, rowID)
			}
			applyPatch(&d.Headers[at], patch)
		case domain.RowCookies:
			at := findCookie(d.Cookies, rowID)
			if at < 0 {
				return notFound(kind, rowID)
			}
			applyCookiePatch(&d.Cookies[at], patch)
		case domain.RowForm:
			at := findFormRow(d.Form, rowID)
			if at < 0 {
				return notFound(kind, rowID)
			}
			applyFormPatch(&d.Form[at], patch)
		default:
			return unknownKind(kind)
		}
		return nil
	})
}

func applyPatch(row *domain.Row, patch RowPatch) {
	if patch.Name != nil {
		row.Name = *patch.Name
	}
	if patch.Value != nil {
		row.Value = *patch.Value
	}
	if patch.Enabled != nil {
		row.Enabled = *patch.Enabled
	}
}

func applyCookiePatch(row *domain.CookieRow, patch RowPatch) {
	if patch.Name != nil {
		row.Name = *patch.Name
	}
	if patch.Value != nil {
		row.Value = *patch.Value
	}
	if patch.Domain != nil {
		row.Domain = *patch.Domain
	}
	if patch.Expires != nil {
		row.Expires = *patch.Expires
	}
	if patch.Secure != nil {
		row.Secure = *patch.Secure
	}
	if patch.HTTPOnly != nil {
		row.HTTPOnly = *patch.HTTPOnly
	}
}

// without returns the rows without the one named — a new slice, so that a change that cannot be
// saved can still be rolled back to the one before it.
func without(rows []domain.Row, id string) []domain.Row {
	out := make([]domain.Row, 0, len(rows))
	for _, row := range rows {
		if row.ID != id {
			out = append(out, row)
		}
	}
	return out
}

func withoutCookie(rows []domain.CookieRow, id string) []domain.CookieRow {
	out := make([]domain.CookieRow, 0, len(rows))
	for _, row := range rows {
		if row.ID != id {
			out = append(out, row)
		}
	}
	return out
}

func withoutForm(rows []domain.FormRow, id string) []domain.FormRow {
	out := make([]domain.FormRow, 0, len(rows))
	for _, row := range rows {
		if row.ID != id {
			out = append(out, row)
		}
	}
	return out
}

func findFormRow(rows []domain.FormRow, id string) int {
	for i, row := range rows {
		if row.ID == id {
			return i
		}
	}
	return -1
}

// applyFormPatch changes what the patch names and leaves the rest. Src and File are separate from
// Value on purpose: the paperclip switches a row between a text and a file, and switching back must
// find the text where it was left.
func applyFormPatch(row *domain.FormRow, patch RowPatch) {
	if patch.Name != nil {
		row.Name = *patch.Name
	}
	if patch.Value != nil {
		row.Value = *patch.Value
	}
	if patch.Enabled != nil {
		row.Enabled = *patch.Enabled
	}
	if patch.Src != nil {
		row.Src = *patch.Src
	}
	if patch.File != nil {
		row.File = *patch.File
	}
}

func findRow(rows []domain.Row, id string) int {
	for i, row := range rows {
		if row.ID == id {
			return i
		}
	}
	return -1
}

func findCookie(rows []domain.CookieRow, id string) int {
	for i, row := range rows {
		if row.ID == id {
			return i
		}
	}
	return -1
}

func unknownKind(kind domain.RowKind) error {
	return domain.Refuse(domain.CodeUnknownList, domain.ErrNotAllowed,
		domain.Args{"list": string(kind)})
}

func notFound(kind domain.RowKind, id string) error {
	return domain.Refuse(domain.CodeUnknownList, domain.ErrNotFound,
		domain.Args{"list": string(kind), "row": id})
}
