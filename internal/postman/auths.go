package postman

import "json-inspector/internal/domain"

// The two names a scheme goes by in the format, and the table that pairs them with this app's
// own. Adding a scheme is a row here and nothing else: neither direction asks what type it is
// holding.

// scheme is one of this app's schemes as the file format writes it: what Postman calls the scheme,
// and what it calls each of its fields. Every disagreement between the two formats is collected
// here, which is what keeps them out of the code that walks a scheme.
type scheme struct {
	Name   string
	Fields map[string]string
}

var schemes = map[domain.AuthType]scheme{
	domain.AuthBearer: {Name: "bearer", Fields: map[string]string{"token": "token"}},
	domain.AuthBasic: {Name: "basic", Fields: map[string]string{
		"username": "username", "password": "password",
	}},
	domain.AuthAPIKey: {Name: "apikey", Fields: map[string]string{
		"key": "key", "value": "value", "place": "in",
	}},
	domain.AuthOAuth2: {Name: "oauth2", Fields: map[string]string{
		"grant": "grant_type", "tokenUrl": "accessTokenUrl", "clientId": "clientId",
		"clientSecret": "clientSecret", "clientAuth": "client_authentication",
		"scope": "scope", "audience": "audience", "place": "addTokenTo", "prefix": "headerPrefix",
	}},
	domain.AuthJWT: {Name: "jwt", Fields: map[string]string{
		"algorithm": "algorithm", "secret": "secret", "payload": "payload",
		"expiresIn": "exp", "place": "addTokenTo", "prefix": "headerPrefix",
	}},
	domain.AuthDigest: {Name: "digest", Fields: map[string]string{
		"username": "username", "password": "password",
	}},
	domain.AuthAWS: {Name: "awsv4", Fields: map[string]string{
		"accessKeyId": "accessKey", "secretAccessKey": "secretKey",
		"sessionToken": "sessionToken", "region": "region", "service": "service",
	}},
}

// typeOf is the app's scheme a file's auth names, and whether the app has it at all.
func typeOf(name string) (domain.AuthType, scheme, bool) {
	for kind, written := range schemes {
		if written.Name == name {
			return kind, written, true
		}
	}
	return "", scheme{}, false
}
