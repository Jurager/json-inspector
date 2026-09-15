package domain

import "time"

// AuthType names how a request authorizes itself.
type AuthType string

const (
	AuthNone AuthType = "none"

	// AuthInherit is «Наследовать»: the level above decides. Only a request in a tree can say it:
	// the command line cannot, because nothing is above it.
	AuthInherit AuthType = "inherit"

	AuthBearer AuthType = "bearer"
	AuthBasic  AuthType = "basic"
	AuthAPIKey AuthType = "apikey"
	AuthOAuth2 AuthType = "oauth2"
	AuthJWT    AuthType = "jwt"
	AuthDigest AuthType = "digest"
	AuthAWS    AuthType = "aws"
)

// The two answers to «Добавить в»: where a scheme puts what it carries. A credential in a query
// string is the same credential, and some servers only read it there.
const (
	PlaceHeader = "header"
	PlaceQuery  = "query"
)

// Auth is what a level authorizes itself with: which scheme, and the answers to the fields that
// scheme declared.
//
// Fields is a map rather than a struct because the answers belong to the scheme, so a scheme
// added tomorrow brings its own fields and nothing here has to learn about them: Scheme.Fields
// is the one place that knows their keys, their order and which of them are secret.
type Auth struct {
	Type   AuthType          `json:"type"`
	Fields map[string]string `json:"fields,omitempty"`
}

// NewAuth is an auth of a scheme, with no field answered yet.
func NewAuth(t AuthType) Auth {
	return Auth{Type: t, Fields: map[string]string{}}
}

// Answer is one answer, absent if nobody gave it.
func (a Auth) Answer(key string) string { return a.Fields[key] }

// OrDefault is one answer, or what the scheme starts that field at when nobody gave one. An
// answer that is there wins even when it is empty: only a field nobody has touched falls
// through to the same table the window reads its starting values from, so the two cannot disagree.
func (a Auth) OrDefault(key string) string {
	if value, given := a.Fields[key]; given {
		return value
	}
	scheme, ok := SchemeFor(a.Type)
	if !ok {
		return ""
	}
	for _, field := range scheme.Fields {
		if field.Key == key {
			return field.Default
		}
	}
	return ""
}

// IsNone is a level that says it sends no credentials. «Наследовать» is not this: it is a level
// that has not answered, which is a different thing from one that answered «нет».
func (a Auth) IsNone() bool { return a.Type == AuthNone }

// Answered is whether a level has said anything about authorization at all. «Нет» and
// «Наследовать» are answers about who authorizes the request rather than credentials, so the
// walk goes past them: «нет» on a folder means "not here", not "stop here".
//
// It reads the type and never the fields: a level that said «нет» keeps the answers it was given
// before changing its mind, and the walk does not read fields either.
func (a *Auth) Answered() bool {
	return a != nil && a.Type != AuthNone && a.Type != AuthInherit && a.Type != ""
}

// Stored is the auth as a tree keeps it: nil for a level nobody has answered anything at, which
// is what an absent one means. «Нет» with answers behind it is kept as itself, so that changing
// one's mind back finds them — the walk reads Auth.Answered either way and goes past.
func (a Auth) Stored() *Auth {
	if !a.Answered() && len(a.Fields) == 0 {
		return nil
	}
	return &a
}

// Normalized is the auth as its scheme would hold it: the answers it has, with the scheme's own
// defaults filled in where nobody gave one.
//
// The answers to other schemes are kept, not thrown away: switching a request from Bearer to
// Basic and back is one gesture, and a token that did not survive it is a token to paste again.
// A scheme reads only its own fields, and an export writes only those.
func (a Auth) Normalized() Auth {
	scheme, ok := SchemeFor(a.Type)
	if !ok {
		return NewAuth(a.Type)
	}
	out := make(map[string]string, len(a.Fields)+len(scheme.Fields))
	for key, value := range a.Fields {
		out[key] = value
	}
	for _, field := range scheme.Fields {
		if _, given := out[field.Key]; !given && field.Default != "" {
			out[field.Key] = field.Default
		}
	}
	return Auth{Type: a.Type, Fields: out}
}

// With returns the auth with one answer set. A copy: the map is shared by every holder of the
// value, and an edit that reached through it would change an auth nobody meant to touch.
func (a Auth) With(key, value string) Auth {
	fields := make(map[string]string, len(a.Fields)+1)
	for k, v := range a.Fields {
		fields[k] = v
	}
	fields[key] = value
	return Auth{Type: a.Type, Fields: fields}
}

// FieldKind is the control a field is drawn with.
type FieldKind string

const (
	FieldText     FieldKind = "text"
	FieldPassword FieldKind = "password"
	FieldTextarea FieldKind = "textarea"
	FieldNumber   FieldKind = "number"
	// FieldSelect is a field drawn as a list of choices.
	FieldSelect FieldKind = "select"
)

// Option is one choice of a field that is drawn as a select.
type Option struct {
	Value string `json:"value"`
	// Label is a message key, not a word: the window is the side that knows the language.
	Label string `json:"label"`
}

// Field is one answer a scheme asks for, and how the window draws the question.
type Field struct {
	Key  string    `json:"key"`
	Kind FieldKind `json:"kind"`
	// Label is a message key. So is Placeholder where it reads as prose, and the examples — a header
	// name, a region — are keys too, because a catalogue with holes in it is worse than a long one.
	Label       string   `json:"label"`
	Placeholder string   `json:"placeholder,omitempty"`
	Hint        string   `json:"hint,omitempty"`
	Options     []Option `json:"options,omitempty"`
	Default     string   `json:"default,omitempty"`
	// Secret is about what happens to the value, not how it is drawn: a secret is masked in the rows
	// a scheme projects and in the copy of the request that is written down. The control is Kind's
	// business, and the two genuinely differ — an API key is typed in the open and still a secret.
	Secret bool `json:"secret,omitempty"`
	// Full is a field that takes the whole width. Its neighbours that are not full pair up two to a
	// row, which is how the design draws them.
	Full bool `json:"full,omitempty"`
	// Mono is a value that is read character by character — a URL, a scope, a region.
	Mono bool `json:"mono,omitempty"`
	// Height is how tall a textarea is drawn, in pixels — per field rather than in the stylesheet,
	// because the design gives each its own: a token is pasted whole, a payload is written over
	// lines. Zero is the height the kind is drawn at by default.
	Height int `json:"height,omitempty"`
	// When is a field only some answers bring with them, so which fields a scheme shows follows from
	// another field's answer rather than from anything the window decides. Absent means always drawn.
	When *Condition `json:"when,omitempty"`
}

// Condition is one field's answer making another field relevant: «while Grant Type is one of
// these».
type Condition struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

// Shown is whether a field is drawn for the answers an authorization holds.
func (f Field) Shown(a Auth) bool {
	if f.When == nil {
		return true
	}
	held := a.OrDefault(f.When.Key)
	for _, value := range f.When.Values {
		if value == held {
			return true
		}
	}
	return false
}

// Scheme is one way of authorizing a request: what it asks for, and how it is offered.
type Scheme struct {
	Type AuthType `json:"type"`
	// Label is a message key for the scheme's name where it is offered. The window holds no map of
	// its own: a scheme named here is named everywhere.
	Label string `json:"label"`
	// Menu is the same name where the scheme sits in the «Ещё» menu, for the schemes the design
	// spells out there and shortens in the control. Empty means the label does for both.
	Menu string `json:"menu,omitempty"`
	// Primary is a scheme that sits in the segmented control. The rest are behind «Ещё», which is
	// where the design puts the ones a request rarely needs.
	Primary bool `json:"primary"`
	// NeedsParent is a scheme that only makes sense where there is a level above: a command line has
	// nothing to inherit from, and offering it there would be offering a choice that means nothing.
	NeedsParent bool    `json:"needsParent,omitempty"`
	Fields      []Field `json:"fields"`
	// Fetches is a scheme whose credential is not typed in and not computed here either: it is asked
	// for, and the window offers to ask and to forget rather than only to fill fields in.
	Fetches bool `json:"fetches,omitempty"`
	// Note is a message key for a line the scheme says about itself: «Нет» and «Наследовать» have
	// nothing to fill in and explain themselves instead, and a scheme whose working is invisible —
	// Digest answers a challenge the server has not sent yet — says so under its fields.
	Note string `json:"note,omitempty"`
}

// authSchemes is the registry: every way this app can authorize a request. Adding one is a row
// here plus its behaviour, and no call path has to learn about it.
//
// The order is the order the window draws them in.
var authSchemes = []Scheme{
	{
		Type: AuthNone, Primary: true, Label: "request.auth.none",
		Note: "request.auth.noteNone",
	},
	{
		Type: AuthInherit, Primary: true, NeedsParent: true, Label: "request.auth.inheritShort",
		Note: "request.auth.noteInherit",
	},
	{
		Type: AuthBearer, Primary: true, Label: "request.auth.bearer",
		Fields: []Field{
			{Key: "prefix", Kind: FieldText, Label: "request.auth.field.prefix",
				Placeholder: "request.auth.ph.bearer", Default: "Bearer", Full: true},
			{Key: "token", Kind: FieldTextarea, Label: "request.auth.field.token",
				Placeholder: "request.auth.ph.token", Secret: true, Full: true, Height: 64},
		},
	},
	{
		Type: AuthBasic, Primary: true, Label: "request.auth.basic",
		Fields: []Field{
			{Key: "username", Kind: FieldText, Label: "request.auth.field.username",
				Placeholder: "request.auth.ph.username"},
			{Key: "password", Kind: FieldPassword, Label: "request.auth.field.password",
				Placeholder: "request.auth.ph.password", Secret: true},
		},
	},
	{
		Type: AuthAPIKey, Primary: true, Label: "request.auth.apikey",
		Fields: []Field{
			{Key: "key", Kind: FieldText, Label: "request.auth.field.key",
				Placeholder: "request.auth.ph.apiKey", Mono: true},
			{Key: "value", Kind: FieldText, Label: "request.auth.field.value",
				Placeholder: "request.auth.ph.apiKeyValue", Secret: true},
			{Key: "place", Kind: FieldSelect, Label: "request.auth.field.addTo",
				Default: "header", Full: true, Options: []Option{
					{Value: "header", Label: "request.auth.place.header"},
					{Value: "query", Label: "request.auth.place.query"},
				}},
		},
	},
	{
		Type: AuthOAuth2, Label: "request.auth.oauth2Short", Menu: "request.auth.oauth2", Fetches: true,
		// The four grants ask for different things — a password grant wants a login, a client
		// credentials grant has no place for one — so the fields that are not always asked for say
		// which answers bring them along.
		Fields: []Field{
			{Key: "grant", Kind: FieldSelect, Label: "request.auth.field.grantType",
				Default: "client_credentials", Full: true, Options: []Option{
					{Value: "client_credentials", Label: "request.auth.grant.clientCredentials"},
					{Value: "authorization_code", Label: "request.auth.grant.authorizationCode"},
					{Value: "implicit", Label: "request.auth.grant.implicit"},
					{Value: "password", Label: "request.auth.grant.password"},
				}},
			// Where the person is sent to say yes. Only the two grants that send them anywhere have it.
			{Key: "authUrl", Kind: FieldText, Label: "request.auth.field.authUrl",
				Placeholder: "request.auth.ph.authUrl", Mono: true, Full: true,
				When: &Condition{Key: "grant", Values: []string{"authorization_code", "implicit"}}},
			// And where a token is exchanged for, which the grant that is given one outright has no use
			// for.
			{Key: "tokenUrl", Kind: FieldText, Label: "request.auth.field.tokenUrl",
				Placeholder: "request.auth.ph.tokenUrl", Mono: true, Full: true,
				When: &Condition{Key: "grant",
					Values: []string{"client_credentials", "authorization_code", "password"}}},
			{Key: "clientId", Kind: FieldText, Label: "request.auth.field.clientId",
				Placeholder: "request.auth.ph.clientId", Full: true},
			{Key: "clientSecret", Kind: FieldPassword, Label: "request.auth.field.clientSecret",
				Placeholder: "request.auth.ph.clientSecret", Secret: true, Full: true,
				When: &Condition{Key: "grant",
					Values: []string{"client_credentials", "authorization_code", "password"}}},
			{Key: "clientAuth", Kind: FieldSelect, Label: "request.auth.field.clientAuth",
				Default: "header", Full: true,
				When: &Condition{Key: "grant",
					Values: []string{"client_credentials", "authorization_code", "password"}},
				Options: []Option{
					{Value: "header", Label: "request.auth.clientAuth.header"},
					{Value: "body", Label: "request.auth.clientAuth.body"},
				}},
			// The resource owner's own credential, which only the grant named after them asks for.
			{Key: "owner", Kind: FieldText, Label: "request.auth.field.owner",
				Placeholder: "request.auth.ph.username", Full: true,
				When: &Condition{Key: "grant", Values: []string{"password"}}},
			{Key: "ownerPassword", Kind: FieldPassword, Label: "request.auth.field.ownerPassword",
				Placeholder: "request.auth.ph.password", Secret: true, Full: true,
				When: &Condition{Key: "grant", Values: []string{"password"}}},
			{Key: "scope", Kind: FieldText, Label: "request.auth.field.scope",
				Placeholder: "request.auth.ph.scope", Mono: true, Full: true},
			{Key: "audience", Kind: FieldText, Label: "request.auth.field.audience",
				Placeholder: "request.auth.ph.audience", Mono: true, Full: true},
			{Key: "place", Kind: FieldSelect, Label: "request.auth.field.addTokenTo",
				Default: "header", Full: true, Options: []Option{
					{Value: "header", Label: "request.auth.place.header"},
					{Value: "query", Label: "request.auth.place.query"},
				}},
			{Key: "prefix", Kind: FieldText, Label: "request.auth.field.headerPrefix",
				Placeholder: "request.auth.ph.bearer", Default: "Bearer", Full: true},
		},
	},
	{
		Type: AuthJWT, Label: "request.auth.jwt", Menu: "request.auth.jwtMenu",
		Fields: []Field{
			{Key: "algorithm", Kind: FieldSelect, Label: "request.auth.field.algorithm",
				Default: "HS256", Full: true, Options: []Option{
					{Value: "HS256", Label: "request.auth.alg.hs256"},
					{Value: "HS384", Label: "request.auth.alg.hs384"},
					{Value: "HS512", Label: "request.auth.alg.hs512"},
					{Value: "RS256", Label: "request.auth.alg.rs256"},
					{Value: "RS384", Label: "request.auth.alg.rs384"},
					{Value: "RS512", Label: "request.auth.alg.rs512"},
				}},
			{Key: "secret", Kind: FieldPassword, Label: "request.auth.field.secret",
				Placeholder: "request.auth.ph.secret", Secret: true, Full: true},
			{Key: "secretEncoding", Kind: FieldSelect, Label: "request.auth.field.secretEncoding",
				Default: "plain", Full: true, Options: []Option{
					{Value: "plain", Label: "request.auth.encoding.plain"},
					{Value: "base64", Label: "request.auth.encoding.base64"},
				}},
			{Key: "payload", Kind: FieldTextarea, Label: "request.auth.field.payload",
				Default: `{ "sub": "1234567890" }`, Hint: "request.auth.hint.payload",
				Full: true, Height: 76},
			{Key: "expiresIn", Kind: FieldNumber, Label: "request.auth.field.expiresIn",
				Placeholder: "request.auth.ph.expiresIn", Default: "3600",
				Hint: "request.auth.hint.expiresIn", Mono: true, Full: true},
			{Key: "joseHeader", Kind: FieldTextarea, Label: "request.auth.field.joseHeader",
				Default: `{ "kid": "key-1" }`, Hint: "request.auth.hint.joseHeader",
				Full: true, Height: 56},
			{Key: "place", Kind: FieldSelect, Label: "request.auth.field.addTokenTo",
				Default: "header", Full: true, Options: []Option{
					{Value: "header", Label: "request.auth.place.header"},
					{Value: "query", Label: "request.auth.place.query"},
				}},
			{Key: "prefix", Kind: FieldText, Label: "request.auth.field.headerPrefix",
				Placeholder: "request.auth.ph.bearer", Default: "Bearer", Full: true},
		},
	},
	{
		Type: AuthDigest, Label: "request.auth.digest",
		Note: "request.auth.noteDigest",
		Fields: []Field{
			{Key: "username", Kind: FieldText, Label: "request.auth.field.username",
				Placeholder: "request.auth.ph.username"},
			{Key: "password", Kind: FieldPassword, Label: "request.auth.field.password",
				Placeholder: "request.auth.ph.password", Secret: true},
		},
	},
	{
		Type: AuthAWS, Label: "request.auth.aws", Menu: "request.auth.awsMenu",
		Fields: []Field{
			{Key: "accessKeyId", Kind: FieldText, Label: "request.auth.field.accessKeyId",
				Placeholder: "request.auth.ph.accessKeyId", Mono: true},
			{Key: "secretAccessKey", Kind: FieldPassword, Label: "request.auth.field.secretAccessKey",
				Placeholder: "request.auth.ph.secretAccessKey", Secret: true},
			{Key: "sessionToken", Kind: FieldPassword, Label: "request.auth.field.sessionToken",
				Placeholder: "request.auth.ph.sessionToken", Hint: "request.auth.hint.sessionToken",
				Secret: true, Full: true},
			{Key: "region", Kind: FieldText, Label: "request.auth.field.region",
				Placeholder: "request.auth.ph.region", Mono: true},
			{Key: "service", Kind: FieldText, Label: "request.auth.field.service",
				Placeholder: "request.auth.ph.service", Mono: true},
		},
	},
}

// AuthRequest is the request a scheme may have to look at before it can answer. Most only carry a
// credential and ignore it; the ones that sign what goes out — AWS over the whole thing, Digest
// over the method and the address — cannot answer without it.
type AuthRequest struct {
	Method  string
	URL     string
	Headers []HeaderPair
	Body    []byte
}

// AuthOutput is what a scheme puts on the request: the headers it adds, and the query parameters it
// adds instead where it was asked to travel that way. The two are not alternatives — a scheme that
// signs a request adds more than one header — which is why both are lists.
type AuthOutput struct {
	Headers []HeaderPair
	Query   []HeaderPair
	// Editable is whether an edit to these rows can be turned back into the scheme's fields. A
	// credential encoded as a whole — a Basic login and password, a signature — cannot: base64 does
	// not come apart into what went into it. Such a row is shown, not offered for editing.
	Editable bool
	// Digest is a credential that cannot be put on the request at all yet, and travels to the engine
	// instead: see DigestCredentials.
	Digest *DigestCredentials `json:"digest,omitempty"`
}

// AuthToken is what the window needs to draw the part of an authorization that is not a field: a
// token somebody else issued, which the user asked for and can throw away.
//
// The token itself is not here. The window has no use for it — it goes on the request, and the
// request never passes through the window — and a value that is never sent is a value that cannot
// leak into a screenshot, a log or an export.
type AuthToken struct {
	// Held is whether there is a token at all, which is what the design says «Нет токена» about.
	Held bool `json:"held"`
	// ExpiresAt is unix milliseconds, and zero when the provider did not say. A token with no expiry
	// is kept for as long as the window is open: the provider has said all it is going to say.
	ExpiresAt int64 `json:"expiresAt,omitempty"`
}

// Expired is a token that was good and no longer is. A provider that named no expiry never expires.
func (t AuthToken) Expired(now time.Time) bool {
	return t.Held && t.ExpiresAt > 0 && now.UnixMilli() >= t.ExpiresAt
}

// DigestCredentials is what a Digest scheme gives the engine rather than the request: the header
// cannot be computed here, because the realm and the nonce arrive in a challenge the server sends
// only after a request has been refused, and the engine is where that answer arrives.
type DigestCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ProjectedRow is a row an authorization put in a list rather than a person: a Bearer token is an
// Authorization header, an API key is a header or a query parameter.
//
// It is written down nowhere — the scheme's fields are what is stored — which is why editing it
// edits the fields, and why it disappears once a person writes a row of that name themselves.
type ProjectedRow struct {
	// Target is the list the row belongs to: the window draws it beside the parameters or beside the
	// headers, wherever the scheme put it.
	Target RowKind `json:"target"`
	Name   string  `json:"name"`
	Value  string  `json:"value"`
	// From is the scheme that put it there, for the note beside it.
	From AuthType `json:"from"`
	// Editable travels from the scheme: see AuthOutput.
	Editable bool `json:"editable"`
}

// Empty is a scheme that put nothing on the request: «нет», «наследовать», a field nobody filled
// in. An empty `Bearer ` is a header the server reads as a mistake, so nothing is the honest
// answer.
func (o AuthOutput) Empty() bool { return len(o.Headers) == 0 && len(o.Query) == 0 }

// AuthSchemes lists every way a request can authorize itself, in the order the window draws them.
func AuthSchemes() []Scheme { return authSchemes }

// SchemeFor is the scheme of a type, and whether the registry knows it at all.
func SchemeFor(t AuthType) (Scheme, bool) {
	for _, scheme := range authSchemes {
		if scheme.Type == t {
			return scheme, true
		}
	}
	return Scheme{}, false
}

// FieldKeys is the keys of a scheme in the order it declared them. The order matters and a map has
// none, so everything that walks a scheme's answers — filling in variables, masking them, drawing
// them — walks this list instead.
func FieldKeys(t AuthType) []string {
	scheme, ok := SchemeFor(t)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(scheme.Fields))
	for _, field := range scheme.Fields {
		keys = append(keys, field.Key)
	}
	return keys
}

// WithDefaults is a scheme nobody has answered anything in yet.
func WithDefaults(t AuthType) Auth { return NewAuth(t).Normalized() }
