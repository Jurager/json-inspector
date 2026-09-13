package draft

import (
	"context"
	"mime"
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
	kind := domain.KindOf(draft.BodyKind)

	// The body is rendered twice from one encoding. The boundary is minted by the first rendering and
	// handed to the second, because a boundary that differed between the two would make the copy that
	// is written down a description of a request that was never sent.
	body, boundary, err := u.renderBody(kind, sent.body, draft.BodyFile, sent.form, "", false)
	if err != nil {
		return Prepared{}, err
	}
	masked, _, err := u.renderBody(kind, stored.body, draft.BodyFile, stored.form, boundary, true)
	if err != nil {
		return Prepared{}, err
	}

	return Prepared{
		Method:        draft.Method,
		URL:           sent.url,
		Headers:       withContentType(withAuth(withCookie(sent.headers, sent.cookie), auth.Type, sent.auth), kind, draft.BodyFile, boundary),
		Body:          body,
		BodyKind:      kind,
		Form:          sent.form,
		BodyFile:      draft.BodyFile,
		MaskedURL:     stored.url,
		MaskedHeaders: withContentType(withAuth(withCookie(stored.headers, stored.cookie), auth.Type, stored.auth), kind, draft.BodyFile, boundary),
		MaskedBody:    masked,
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

// withContentType is where the format chosen in the Body popover becomes the header that declares
// it. A row of the same name the user wrote themselves wins, for the reason it wins over the Auth
// chip and one more: a written Content-Type is the only way to send a type the kind cannot name, and
// this app's own Accept is a vendor one, so that is the expected case rather than the exotic one.
func withContentType(headers []domain.HeaderPair, kind domain.BodyKind, file string, boundary string) []domain.HeaderPair {
	value, ok := contentTypeFor(kind, file, boundary)
	if !ok {
		return headers
	}
	for _, header := range headers {
		if strings.EqualFold(header.Name, "Content-Type") {
			return headers
		}
	}
	return append(headers, domain.HeaderPair{Name: "Content-Type", Value: value})
}

// contentTypeFor is the header a body of this kind declares, and whether it declares one at all.
//
// Raw answers with nothing on purpose. Every draft written before there were kinds is raw, and the
// app has always sent those without a Content-Type; naming one here would change what is already
// stored puts on the wire. Raw is also the kind that promises nothing — its whole point is to be the
// escape hatch — and a user who wants text/plain writes it once and it sticks, because a written
// header wins.
func contentTypeFor(kind domain.BodyKind, file string, boundary string) (string, bool) {
	switch kind {
	case domain.BodyJSON:
		return "application/json", true
	case domain.BodyXML:
		return "application/xml", true
	case domain.BodyForm:
		// The boundary is the one the body was actually written with, which is why it is handed in
		// rather than minted here: two boundaries would be a header describing nothing.
		return mime.FormatMediaType("multipart/form-data", map[string]string{"boundary": boundary}), true
	case domain.BodyBinary:
		return typeOfFile(file), true
	default:
		return "", false
	}
}
