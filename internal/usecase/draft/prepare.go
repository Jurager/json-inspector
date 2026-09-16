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
func (u *UseCase) prepare(
	ctx context.Context,
	draft domain.Draft,
	inherits *domain.Auth,
) (Prepared, error) {
	auth := authToApply(draft.Auth, inherits)

	raw := collect(draft)
	// An inherited authorization did not come from the draft, and it is substituted like everything
	// else: a `{{token}}` in the collection's Bearer is filled in on the way out and left as a mask in
	// the copy that is written down.
	raw.auth = auth
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

	// The authorization is worked out from each rendering of the fields, so the copy that is written
	// down carries masked credentials rather than the ones that went out. A scheme that has to hold a
	// conversation to answer — a token to fetch, a challenge to meet — has to recognise the second
	// call as the same one and not hold it twice: same scheme, same answers, same request.
	sentAuth, err := u.auth.Materialize(ctx, sent.auth,
		authRequest(draft.Method, sent.url, sent.headers, body))
	if err != nil {
		return Prepared{}, err
	}
	storedAuth, err := u.auth.Materialize(ctx, stored.auth,
		authRequest(draft.Method, stored.url, stored.headers, masked))
	if err != nil {
		return Prepared{}, err
	}

	// A credential that cannot be put on the request yet goes to the engine instead of onto the
	// request — unless the user wrote an Authorization row themselves, which is the more precise
	// answer and not one a challenge should be answered over.
	digest := sentAuth.Digest
	if digest != nil && hasHeader(sent.headers, "Authorization") {
		digest = nil
	}

	sentURL, sentHeaders := withAuth(sent.url, sent.headers, sentAuth)
	storedURL, storedHeaders := withAuth(stored.url, stored.headers, storedAuth)

	return Prepared{
		Method: draft.Method,
		URL:    sentURL,
		Headers: withContentType(withCookie(sentHeaders, sent.cookie), kind, draft.BodyFile,
			boundary),
		Body:      body,
		BodyKind:  kind,
		Form:      sent.form,
		BodyFile:  draft.BodyFile,
		MaskedURL: storedURL,
		MaskedHeaders: withContentType(withCookie(storedHeaders, stored.cookie), kind, draft.BodyFile,
			boundary),
		MaskedBody: masked,
		Cookies:    draft.Cookies,
		Digest:     digest,
	}, nil
}

func authRequest(
	method, rawURL string,
	headers []domain.HeaderPair,
	body string,
) domain.AuthRequest {
	return domain.AuthRequest{Method: method, URL: rawURL, Headers: headers, Body: []byte(body)}
}

// authToApply is the authorization this request goes out with: the one the draft chose, or — when
// it chose «Inherit» — the one the levels above it answered with, which the caller resolved.
// Nothing above a request is «None»: a request that inherits from nothing sends no credentials.
func authToApply(chosen domain.Auth, inherits *domain.Auth) domain.Auth {
	if chosen.Type != domain.AuthInherit {
		return chosen
	}
	if inherits == nil {
		return domain.NewAuth(domain.AuthNone)
	}
	return *inherits
}

// A row of the same name the user wrote wins: an explicit row is the more precise answer, and two
// Authorization headers are a request no server reads the way the window drew it. The query side
// works on the URL being sent — a projection into the draft's rows dies on the next keystroke.
func withAuth(
	rawURL string,
	headers []domain.HeaderPair,
	out domain.AuthOutput,
) (string, []domain.HeaderPair) {
	for _, pair := range out.Headers {
		if hasHeader(headers, pair.Name) {
			continue
		}
		headers = append(headers, pair)
	}
	for _, pair := range out.Query {
		if hasParam(rawURL, pair.Name) {
			continue
		}
		rawURL = appendParam(rawURL, pair)
	}
	return rawURL, headers
}

func hasHeader(headers []domain.HeaderPair, name string) bool {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return true
		}
	}
	return false
}

func hasParam(rawURL, name string) bool {
	for _, row := range paramsFromURL(rawURL) {
		if row.Name == name {
			return true
		}
	}
	return false
}

// appendParam puts one more parameter on an address, before the fragment if it has one: a fragment
// is not a parameter, and a query string after it is a query string to nothing.
func appendParam(rawURL string, pair domain.HeaderPair) string {
	fragment := ""
	if hash := strings.IndexByte(rawURL, '#'); hash >= 0 {
		rawURL, fragment = rawURL[:hash], rawURL[hash:]
	}
	separator := "?"
	if strings.ContainsRune(rawURL, '?') {
		separator = "&"
	}
	return rawURL + separator + encodeQuery(pair.Name) + "=" + encodeQuery(pair.Value) + fragment
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
// chip and one more: a written Content-Type is the only way to send a type the kind cannot name,
// and this app's own Accept is a vendor one, so that is the expected case rather than the exotic
// one.
func withContentType(
	headers []domain.HeaderPair,
	kind domain.BodyKind,
	file string,
	boundary string,
) []domain.HeaderPair {
	value, ok := contentTypeFor(kind, file, boundary)
	if !ok {
		return headers
	}
	if hasHeader(headers, "Content-Type") {
		return headers
	}
	return append(headers, domain.HeaderPair{Name: "Content-Type", Value: value})
}

// Raw answers with nothing on purpose: every draft written before there were kinds is raw and has
// always gone out without a Content-Type, and naming one would change what already stored requests
// put on the wire. A written header wins, so a text/plain typed once sticks.
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
