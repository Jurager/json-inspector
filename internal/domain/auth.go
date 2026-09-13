package domain

import "encoding/base64"

// AuthType is how a request authorizes itself.
type AuthType string

const (
	AuthNone AuthType = "none"

	// AuthInherit is «Наследовать»: the level above decides. Only a request that sits in a tree can
	// say it — a collection, a folder or a node of one — and the command line cannot, because
	// nothing is above it.
	AuthInherit AuthType = "inherit"

	AuthBearer AuthType = "bearer"
	AuthBasic  AuthType = "basic"
	AuthOAuth2 AuthType = "oauth2"
)

// Auth is what a level authorizes its requests with. Token is the whole of it for both kinds that
// have one: a Basic credential is written `user:password` in the same field a Bearer token is typed
// in, because one field is what both the chip and the collection tab have room for.
type Auth struct {
	Type  AuthType `json:"type"`
	Token string   `json:"token"`
}

// Header is the header this auth puts on the wire, and whether it puts one there at all: «Нет» and
// «Наследовать» are answers about who authorizes the request rather than credentials, and a token
// that resolves to nothing is not one either — an empty `Bearer ` would be a header the server reads
// as a mistake.
//
// OAuth 2 has no flow behind it yet, so it is an answer of the same kind as «Нет».
func (a Auth) Header() (name string, value string, ok bool) {
	if a.Token == "" {
		return "", "", false
	}
	switch a.Type {
	case AuthBearer:
		return "Authorization", "Bearer " + a.Token, true
	case AuthBasic:
		return "Authorization", "Basic " + base64.StdEncoding.EncodeToString([]byte(a.Token)), true
	}
	return "", "", false
}
