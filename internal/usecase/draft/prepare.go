package draft

import (
	"context"
	"strings"

	"json-inspector/internal/domain"
)

// prepare fills the draft's variables in twice: once with their values, which is what goes out, and
// once with a secret left as its mask, which is what everything that outlives the send gets to see.
// The window never holds either — a secret's value has not left this side since it was typed in.
func (u *UseCase) prepare(ctx context.Context, draft domain.Draft, inherits *domain.Auth) (Prepared, error) {
	auth := authToApply(draft.Auth, inherits)

	raw := collect(draft)
	// An inherited authorization did not come from the draft, and it is substituted like everything
	// else: a `{{token}}` in the collection's Bearer is filled in on the way out and left as a mask in
	// the copy that is written down.
	raw.auth = auth.Token
	texts := raw.texts()

	live, err := u.vars.SubstituteTexts(ctx, texts, false)
	if err != nil {
		return Prepared{}, err
	}
	hidden, err := u.vars.SubstituteTexts(ctx, texts, true)
	if err != nil {
		return Prepared{}, err
	}

	sent, stored := raw.putBack(live), raw.putBack(hidden)
	return Prepared{
		Method:        draft.Method,
		URL:           sent.url,
		Headers:       withAuth(withCookie(sent.headers, sent.cookie), auth.Type, sent.auth),
		Body:          sent.body,
		MaskedURL:     stored.url,
		MaskedHeaders: withAuth(withCookie(stored.headers, stored.cookie), auth.Type, stored.auth),
		MaskedBody:    stored.body,
		Cookies:       draft.Cookies,
	}, nil
}

// authToApply is the authorization this request goes out with: the one the draft chose, or — when
// it chose «Наследовать» — the one the levels above it answered with, which the caller resolved.
// Nothing above a request is «нет»: a request that inherits from nothing sends no credentials.
func authToApply(chosen domain.Auth, inherits *domain.Auth) domain.Auth {
	if chosen.Type != domain.AuthInherit {
		return chosen
	}
	if inherits == nil {
		return domain.Auth{Type: domain.AuthNone}
	}
	return *inherits
}

// withAuth is where the choice in the Auth chip becomes the header that carries it. A row of the
// same name the user wrote themselves wins: an explicit header is the more precise answer, and two
// Authorization headers are a request no server reads the way the window drew it.
func withAuth(headers []domain.HeaderPair, kind domain.AuthType, token string) []domain.HeaderPair {
	name, value, ok := (domain.Auth{Type: kind, Token: token}).Header()
	if !ok {
		return headers
	}
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return headers
		}
	}
	return append(headers, domain.HeaderPair{Name: name, Value: value})
}

// withCookie is where the jar becomes the header that carries it. The rows travel beside it, so
// that a record can hand the jar back to the draft it came from.
func withCookie(headers []domain.HeaderPair, cookie string) []domain.HeaderPair {
	if cookie == "" {
		return headers
	}
	return append(headers, domain.HeaderPair{Name: "Cookie", Value: cookie})
}
