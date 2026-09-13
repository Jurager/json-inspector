package domain

import "testing"

// What the four choices in the chip put on the wire. Two of them put nothing there, and that is the
// point of the test: «Нет» and «Наследовать» are answers about who authorizes the request, not
// credentials, and a request carrying `Bearer ` with nothing after it is a request a server refuses.
func TestAuthHeader(t *testing.T) {
	cases := []struct {
		name  string
		auth  Auth
		want  string
		found bool
	}{
		{"bearer", Auth{Type: AuthBearer, Token: "abc123"}, "Bearer abc123", true},
		{"basic", Auth{Type: AuthBasic, Token: "user:pass"}, "Basic dXNlcjpwYXNz", true},
		{"нет", Auth{Type: AuthNone, Token: "abc123"}, "", false},
		{"наследовать", Auth{Type: AuthInherit, Token: "abc123"}, "", false},
		{"oauth2", Auth{Type: AuthOAuth2, Token: "abc123"}, "", false},
		{"пустой токен", Auth{Type: AuthBearer}, "", false},
		{"переменная не подставилась", Auth{Type: AuthBearer, Token: "{{token}}"}, "Bearer {{token}}", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			name, value, ok := c.auth.Header()
			if ok != c.found {
				t.Fatalf("Header() found = %v, want %v", ok, c.found)
			}
			if !c.found {
				return
			}
			if name != "Authorization" || value != c.want {
				t.Errorf("Header() = %q: %q, want %q", name, value, c.want)
			}
		})
	}
}
