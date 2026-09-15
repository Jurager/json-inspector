package vars

import (
	"strings"
	"testing"

	"json-inspector/internal/domain"
)

// fakeResolver is the environment these tests resolve against: one plain variable, one global and
// one secret, so that "unknown", "global" and "masked" are all reachable.
func fakeResolver(name string) (domain.Resolution, bool) {
	switch name {
	case "host":
		return domain.Resolution{Value: "api.example.com", Source: "env", Kind: domain.VariableText}, true
	case "page":
		return domain.Resolution{Value: "2", Source: "global", Kind: domain.VariableText}, true
	case "token":
		return domain.Resolution{Value: "s3cret", Source: "env", Kind: domain.VariableSecret}, true
	}
	return domain.Resolution{}, false
}

func TestParseTokens(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []Token
	}{
		{"empty", "", nil},
		{"prose", "no tokens here", nil},
		{"one", "{{host}}", []Token{{Start: 0, End: 8, Name: "host", Raw: "{{host}}"}}},
		{
			"spaces inside",
			"before {{ host }} after",
			[]Token{{Start: 7, End: 17, Name: "host", Raw: "{{ host }}"}},
		},
		{
			"two, offsets past the first",
			"{{host}}{{page}}",
			[]Token{
				{Start: 0, End: 8, Name: "host", Raw: "{{host}}"},
				{Start: 8, End: 16, Name: "page", Raw: "{{page}}"},
			},
		},
		{
			"escaped then real",
			`\{{host}} {{page}}`,
			[]Token{{Start: 10, End: 18, Name: "page", Raw: "{{page}}"}},
		},
		{"only an escape", `\{{host}}`, nil},
		{"a JSON payload is not a token", `{{"a": 1}}`, nil},
		{"empty name", "{{}}", nil},
		{"dash in the name", "{{a-b}}", nil},
		{"leading digit", "{{9a}}", nil},
		{"unclosed", "{{host", nil},
		{
			"unclosed after a real token",
			"{{host}} and {{page",
			[]Token{{Start: 0, End: 8, Name: "host", Raw: "{{host}}"}},
		},
		{
			"a payload before a real token",
			`{{"a": 1}} {{host}}`,
			[]Token{{Start: 11, End: 19, Name: "host", Raw: "{{host}}"}},
		},
		{
			"the name is trimmed, newlines included",
			"{{\nhost\n}}",
			[]Token{{Start: 0, End: 10, Name: "host", Raw: "{{\nhost\n}}"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseTokens(tc.text)
			assertTokens(t, got, tc.want)
		})
	}
}

// assertTokens compares the whole token sequence: offsets decide where the highlight layer draws,
// so a token that is found but misplaced is a failure too.
func assertTokens(t *testing.T, got, want []Token) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ParseTokens = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSubstitute(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"plain text is returned as it is", "no tokens", "no tokens"},
		{"known", "{{host}}/articles", "api.example.com/articles"},
		{"the name is trimmed", "{{ host }}", "api.example.com"},
		{"a secret is substituted too: this is the send path", "Bearer {{token}}", "Bearer s3cret"},
		{
			"unknown is left exactly as written",
			"Bearer {{nope}}",
			"Bearer {{nope}}",
		},
		{
			"an escaped opener is not a token",
			`\{{host}}`,
			`\{{host}}`,
		},
		{"a JSON payload travels untouched", `{"a": "{{host}}"}`, `{"a": "api.example.com"}`},
		{"a payload with no name is untouched", `{{"a": 1}}`, `{{"a": 1}}`},
		{"several", "{{host}}:{{page}}?", "api.example.com:2?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Substitute(tc.text, fakeResolver); got != tc.want {
				t.Errorf("Substitute(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestSubstituteMasked(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"known text value", "{{host}}", "api.example.com"},
		{"a global value is not a secret", "{{page}}", "2"},
		{"a secret leaves as the mask", "Bearer {{token}}", "Bearer " + SecretMask},
		{"unknown is left exactly as written", "{{nope}}", "{{nope}}"},
		{"escaped", `\{{token}}`, `\{{token}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SubstituteMasked(tc.text, fakeResolver); got != tc.want {
				t.Errorf("SubstituteMasked(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestMissing(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"nothing missing", "{{host}}", []string{}},
		{"one", "{{nope}}", []string{"nope"}},
		{
			"deduplicated in order of first appearance",
			"{{b}} {{a}} {{b}} {{c}} {{a}}",
			[]string{"b", "a", "c"},
		},
		{"known names are not reported", "{{nope}} {{host}}", []string{"nope"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Missing(tc.text, fakeResolver)
			if len(got) != len(tc.want) {
				t.Fatalf("Missing(%q) = %v, want %v", tc.text, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("Missing(%q) = %v, want %v (index %d)", tc.text, got, tc.want, i)
				}
			}
		})
	}
}

func TestSegments(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []Segment
	}{
		{"empty", "", nil},
		{
			"no tokens is one plain segment",
			"just text",
			[]Segment{{Text: "just text", Start: 0}},
		},
		{
			"text on both sides",
			"before {{ host }} after",
			[]Segment{
				{Text: "before ", Start: 0},
				{Text: "{{ host }}", TokenName: "host", Start: 7},
				{Text: " after", Start: 17},
			},
		},
		{
			"a token at the very start",
			"{{host}}/x",
			[]Segment{
				{Text: "{{host}}", TokenName: "host", Start: 0},
				{Text: "/x", Start: 8},
			},
		},
		{
			"a token at the very end",
			"x/{{host}}",
			[]Segment{
				{Text: "x/", Start: 0},
				{Text: "{{host}}", TokenName: "host", Start: 2},
			},
		},
		{
			"unknown and escaped spans are plain text",
			`{{nope}} \{{host}}`,
			[]Segment{
				{Text: "{{nope}}", TokenName: "nope", Start: 0},
				{Text: ` \{{host}}`, Start: 8},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Segments(tc.text)
			if len(got) != len(tc.want) {
				t.Fatalf("Segments(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("segment %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestSegmentsRebuildText is the invariant the highlight layer depends on: the segments are painted
// over a real input, so a character dropped between them would show up as misaligned text.
func TestSegmentsRebuildText(t *testing.T) {
	texts := []string{
		"",
		"plain",
		"{{host}}",
		"{{ host }}/articles?q={{page}}",
		`\{{host}} {{nope}}`,
		`{"a": 1, "b": "{{token}}"}`,
		"{{a}}{{b}}{{c}}",
		"\n\n{{host}}\r\n",
		"{{unclosed",
	}
	for _, text := range texts {
		var b strings.Builder
		last := 0
		for _, s := range Segments(text) {
			if s.Start != last {
				t.Errorf("Segments(%q): a segment starts at %d, want %d", text, s.Start, last)
			}
			b.WriteString(s.Text)
			last = s.Start + len(s.Text)
		}
		if got := b.String(); got != text {
			t.Errorf("Segments(%q) rebuilds %q", text, got)
		}
	}
}
