package draft

// The Auth chip as the draft keeps it: what a level authorizes itself with, and the rows a
// scheme projects into the lists beside the ones a person wrote.

import (
	"context"

	"json-inspector/internal/domain"
)

// SetAuth is the whole of what a level authorizes itself with, sent whole: the scheme and the
// answers to the fields that scheme asks for. What arrives is normalized against the scheme, so a
// token left over from the scheme before it cannot travel as an answer nobody asked for.
func (u *UseCase) SetAuth(ctx context.Context, id domain.DraftID, auth domain.Auth) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		d.Auth = auth.Normalized()
		return nil
	})
}

// PatchDerived is an edit to a row the authorization put in a list: the token in the header, the
// value of an API key. The row is not stored — it is what the scheme's fields come to — so the edit
// goes the other way, into the field behind it, and the row the window draws next is the same one
// worked out again. A scheme that cannot take the edit refuses it, and the window does not offer
// one: see domain.AuthOutput.Editable.
func (u *UseCase) PatchDerived(
	ctx context.Context,
	id domain.DraftID,
	target domain.RowKind,
	name string,
	value string,
) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		auth, err := u.auth.Absorb(d.Auth, target, name, value)
		if err != nil {
			return err
		}
		d.Auth = auth.Normalized()
		return nil
	})
}

// ObtainAuth asks whoever issues the token to issue one, and ForgetAuth throws it away and lets the
// next send ask for another. Neither changes what the user answered: what is being got and dropped
// is what those answers currently come to, not the answers themselves.
func (u *UseCase) ObtainAuth(ctx context.Context, id domain.DraftID) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		return u.auth.Obtain(ctx, d.Auth)
	})
}

func (u *UseCase) ForgetAuth(ctx context.Context, id domain.DraftID) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		u.auth.Forget(d.Auth)
		return nil
	})
}

// RemoveDerived is a projected row deleted from the list it was drawn in. There is no row to delete
// — the row is what the scheme's fields come to — so what the deletion reaches is what put it
// there: the request stops authorizing itself. A row a person wrote is removed as a row, by its id,
// and never comes here.
func (u *UseCase) RemoveDerived(ctx context.Context, id domain.DraftID) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		// The answers stay: the request stops authorizing itself, which is what deleting the row
		// asked for, and emptying the fields as well would throw away more than the gesture meant.
		d.Auth = domain.Auth{Type: domain.AuthNone, Fields: d.Auth.Fields}
		return nil
	})
}

// Held is what a scheme that fetches has at this moment, handed the same way the projection is: a
// request inside a collection takes its authorization from the tree, and the answer travels in
// rather than being walked for here.
func (u *UseCase) Held(draft domain.Draft, inherits *domain.Auth) *domain.AuthToken {
	auth := authToApply(draft.Auth, inherits)
	scheme, ok := domain.SchemeFor(auth.Type)
	if !ok || !scheme.Fetches {
		return nil
	}
	token := u.auth.Held(auth)
	return &token
}

// Project is what the draft's authorization puts in the parameter and header lists, with what the
// levels above answered handed in. Whoever can walk a tree passes its answer here; a request that
// stands on its own passes nothing, and `inherits` going unused is that case rather than a mistake.
//
// The two callers are the draft describing itself — where nothing above it is known — and the layer
// that knows both, which asks again with the inherited answer in hand.
func (u *UseCase) Project(
	ctx context.Context,
	draft domain.Draft,
	inherits *domain.Auth,
) ([]domain.ProjectedRow, error) {
	return u.projected(ctx, draft, inherits)
}

// projected is the rows the request's authorization puts in the parameter and header lists. The
// window draws them beside the rows a person wrote, because that is where they end up: a Bearer
// token is an Authorization header, and a list that did not show it would be a list of the request
// without the credential.
//
// A row a person wrote under that name is the better answer and the projected one is dropped — the
// same rule sending follows, so the list the window draws is the list that goes out.
//
// The fields are read as they are, `{{tokens}}` and all: this is not the request being sent but the
// request being described, and the window paints a token the way it paints one in any other row.
func (u *UseCase) projected(
	ctx context.Context,
	draft domain.Draft,
	inherits *domain.Auth,
) ([]domain.ProjectedRow, error) {
	auth := authToApply(draft.Auth, inherits)
	out, err := u.auth.Project(auth, authRequest(draft.Method, draft.URL, nil, draft.Body))
	if err != nil {
		return nil, err
	}

	// The comparison is made against what actually goes out, which is the same list sending compares
	// against: enabled rows with a name in them. A row the user switched off is not competing with
	// anything, and dropping the projection over it would hide a credential that is on its way.
	sent := collect(draft)

	rows := []domain.ProjectedRow{}
	for _, pair := range out.Headers {
		if !hasHeader(sent.headers, pair.Name) {
			rows = append(rows, domain.ProjectedRow{
				Target: domain.RowHeaders, Name: pair.Name, Value: pair.Value,
				From: auth.Type, Editable: out.Editable,
			})
		}
	}
	for _, pair := range out.Query {
		if !hasParam(draft.URL, pair.Name) {
			rows = append(rows, domain.ProjectedRow{
				Target: domain.RowParams, Name: pair.Name, Value: pair.Value,
				From: auth.Type, Editable: out.Editable,
			})
		}
	}
	return rows, nil
}
