package domain

import "testing"

// The registry is the one place that knows what a scheme asks for, and the window draws whatever it
// says. A row that contradicts itself — a select with no choices, a field with no name — is a hole
// in that drawing rather than a mistake anybody would notice at the point of use.
func TestEverySchemeDescribesItself(t *testing.T) {
	seen := map[AuthType]bool{}
	primary := 0

	for _, scheme := range AuthSchemes() {
		t.Run(string(scheme.Type), func(t *testing.T) {
			if scheme.Type == "" {
				t.Fatal("a scheme with no type cannot be chosen")
			}
			if seen[scheme.Type] {
				t.Fatalf("two schemes answer to %q", scheme.Type)
			}
			seen[scheme.Type] = true
			if scheme.Primary {
				primary++
			}

			// A scheme with nothing to fill in has to explain itself instead, and one with fields has
			// no business being empty either.
			if len(scheme.Fields) == 0 && scheme.Note == "" {
				t.Error("a scheme with no fields and nothing to say is a blank panel")
			}

			keys := map[string]bool{}
			for _, field := range scheme.Fields {
				if field.Key == "" || field.Label == "" {
					t.Errorf("a field with no key or no label cannot be drawn: %+v", field)
				}
				if keys[field.Key] {
					t.Errorf("two fields answer to %q", field.Key)
				}
				keys[field.Key] = true

				if field.Kind == FieldSelect {
					if len(field.Options) == 0 {
						t.Errorf("field %q is a select with nothing to choose", field.Key)
					}
					if field.Default != "" && !hasOption(field.Options, field.Default) {
						t.Errorf("field %q starts on %q, which is not one of its choices", field.Key, field.Default)
					}
				} else if len(field.Options) > 0 {
					t.Errorf("field %q carries choices but is drawn as %q", field.Key, field.Kind)
				}
				if field.Default != "" && field.Kind == FieldSelect && field.Secret {
					t.Errorf("field %q is a secret with a value filled in for it", field.Key)
				}
			}

			if scheme.NeedsParent && scheme.Type != AuthInherit {
				t.Errorf("%q says it needs a level above it, which only «Наследовать» means", scheme.Type)
			}
		})
	}

	// The segmented control is drawn from the same list, and every scheme in the «Ещё» menu is one
	// nobody reached by accident: a registry that named none of them primary would have no control.
	if primary == 0 {
		t.Error("every scheme is behind «Ещё»: there would be no segmented control at all")
	}
}

// WithDefaults is what the window opens a scheme on, so a select never starts blank: an empty
// select is a question the user has to answer before the scheme means anything.
func TestDefaultsAreFilledIn(t *testing.T) {
	auth := WithDefaults(AuthBearer)
	if auth.Type != AuthBearer {
		t.Errorf("type = %q, want the one asked for", auth.Type)
	}
	if auth.Get("prefix") != "Bearer" {
		t.Errorf("prefix = %q, want the scheme's own default", auth.Get("prefix"))
	}
	if _, ok := auth.Fields["token"]; ok {
		t.Error("a field with no default belongs absent, not empty: empty is an answer nobody gave")
	}

	// Every choice of a select has a first one, or there is nothing to start on.
	for _, scheme := range AuthSchemes() {
		filled := WithDefaults(scheme.Type)
		for _, field := range scheme.Fields {
			if field.Kind == FieldSelect && filled.Get(field.Key) == "" {
				t.Errorf("%s: select %q starts with nothing chosen", scheme.Type, field.Key)
			}
		}
	}
}

// With is a copy. Every holder of an Auth shares its map, and an edit that reached through it would
// change an authorization nobody meant to touch — a level's own while a request is being edited.
func TestWithDoesNotReachThrough(t *testing.T) {
	original := NewAuth(AuthBearer).With("token", "first")
	edited := original.With("token", "second")

	if original.Get("token") != "first" {
		t.Errorf("original = %q, want it left alone", original.Get("token"))
	}
	if edited.Get("token") != "second" {
		t.Errorf("edited = %q, want the new answer", edited.Get("token"))
	}
}

// Normalized is what the scheme would hold, and the window sends what it has: switching a request
// from Bearer to Basic leaves a token behind, and a select nobody touched has no answer at all.
// Neither is an error to report — both are settled here, before anything reads the answers.
func TestNormalizedKeepsOnlyWhatTheSchemeAsksFor(t *testing.T) {
	// A token carried over to a scheme that has no field for it.
	carried := Auth{Type: AuthBasic, Fields: map[string]string{
		"token": "left-over", "username": "user", "nothing-asks-for-this": "x",
	}}
	normalized := carried.Normalized()

	for _, key := range []string{"token", "nothing-asks-for-this"} {
		if _, ok := normalized.Fields[key]; ok {
			t.Errorf("%q survived a scheme that does not ask for it", key)
		}
	}
	if normalized.Get("username") != "user" {
		t.Errorf("username = %q, want the answer that was given", normalized.Get("username"))
	}
	// The one thing the window never sends is what a field starts at, because the window is not the
	// side that knows: a select has a first choice whether or not anyone picked one.
	filled := NewAuth(AuthAPIKey).Normalized()
	if filled.Get("place") != "header" {
		t.Errorf("place = %q, want the scheme's own first choice", filled.Get("place"))
	}

	// A scheme nobody has heard of normalizes to nothing rather than to itself with its answers
	// kept: there is no field list to check them against.
	stranger := Auth{Type: "nobody-has-this", Fields: map[string]string{"token": "x"}}
	if len(stranger.Normalized().Fields) != 0 {
		t.Errorf("an unknown scheme kept %v", stranger.Normalized().Fields)
	}
}

// FieldKeys is the order a scheme declared, because a map has none and everything that walks a
// scheme's answers — filling variables in, masking them, writing them to a file — has to walk the
// same list in the same order.
func TestFieldKeysKeepTheDeclaredOrder(t *testing.T) {
	for _, scheme := range AuthSchemes() {
		keys := FieldKeys(scheme.Type)
		if len(keys) != len(scheme.Fields) {
			t.Fatalf("%s: %d keys for %d fields", scheme.Type, len(keys), len(scheme.Fields))
		}
		for i, key := range keys {
			if key != scheme.Fields[i].Key {
				t.Errorf("%s: key %d is %q, want %q", scheme.Type, i, key, scheme.Fields[i].Key)
			}
		}
	}
	if keys := FieldKeys(AuthType("nobody-has-this")); keys != nil {
		t.Errorf("an unknown scheme answered with %v, want nothing", keys)
	}
}

func hasOption(options []Option, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}
