package draft

import (
	"context"

	"json-inspector/internal/domain"
)

// The answers arrive normalized: a token left over from the scheme before it cannot travel as an
// answer nobody asked for.
func (u *UseCase) SetAuth(ctx context.Context, id domain.DraftID, auth domain.Auth) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		d.Auth = auth.Normalized()
		return nil
	})
}

// A projected row is not stored — it is what the scheme's fields come to — so an edit goes the
// other way, into the field behind it. A scheme that cannot take the edit refuses it:
// see domain.AuthOutput.Editable.
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

// Neither changes what the user answered: what is dropped is what those answers come to now, not
// the answers themselves.
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

// There is no row to delete — the row is what the scheme's fields come to — so what the deletion
// reaches is what put it there: the request stops authorizing itself.
func (u *UseCase) RemoveDerived(ctx context.Context, id domain.DraftID) (State, error) {
	return u.result(ctx, id, func(d *domain.Draft) error {
		// The answers stay: the request stops authorizing itself, which is what deleting the row
		// asked for, and emptying the fields as well would throw away more than the gesture meant.
		d.Auth = domain.Auth{Type: domain.AuthNone, Fields: d.Auth.Fields}
		return nil
	})
}

// The authorization travels in rather than being walked for here: a request inside a collection
// takes it from the tree, and a draft cannot walk one.
func (u *UseCase) Held(draft domain.Draft, inherits *domain.Auth) *domain.AuthToken {
	auth := authToApply(draft.Auth, inherits)
	scheme, ok := domain.SchemeFor(auth.Type)
	if !ok || !scheme.Fetches {
		return nil
	}
	token := u.auth.Held(auth)
	return &token
}

// Whoever can walk a tree passes its answer here; a request that stands on its own passes nothing,
// and `inherits` going unused is that case rather than a mistake.
func (u *UseCase) Project(
	ctx context.Context,
	draft domain.Draft,
	inherits *domain.Auth,
) ([]domain.ProjectedRow, error) {
	return u.projected(ctx, draft, inherits)
}

// The rows the authorization puts in the parameter and header lists are drawn beside the ones a
// person wrote, because that is where they end up: a Bearer token is an Authorization header.
//
// A written row under that name drops the projected one — the rule sending follows — so the list
// the window draws is the list that goes out. The fields stay as typed, `{{tokens}}` and all:
// this is the request being described, not the one being sent.
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

	// The comparison is against what actually goes out: a row the user switched off is not competing
	// with anything, and dropping the projection over it would hide a credential that is on its way.
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
