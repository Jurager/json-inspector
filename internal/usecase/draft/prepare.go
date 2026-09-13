package draft

import (
	"context"

	"json-inspector/internal/domain"
)

// prepare fills the draft's variables in twice: once with their values, which is what goes out, and
// once with a secret left as its mask, which is what everything that outlives the send gets to see.
// The window never holds either — a secret's value has not left this side since it was typed in.
func (u *UseCase) prepare(ctx context.Context, draft domain.Draft) (Prepared, error) {
	raw := collect(draft)
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
		Headers:       withCookie(sent.headers, sent.cookie),
		Body:          sent.body,
		MaskedURL:     stored.url,
		MaskedHeaders: withCookie(stored.headers, stored.cookie),
		MaskedBody:    stored.body,
		Cookies:       draft.Cookies,
	}, nil
}

// withCookie is where the jar becomes the header that carries it. The rows travel beside it, so
// that a record can hand the jar back to the draft it came from.
func withCookie(headers []domain.HeaderPair, cookie string) []domain.HeaderPair {
	if cookie == "" {
		return headers
	}
	return append(headers, domain.HeaderPair{Name: "Cookie", Value: cookie})
}
