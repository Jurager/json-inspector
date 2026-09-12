package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"json-inspector/internal/vars"
)

// This corpus was dumped from the TypeScript implementation (frontend/tools/dump-fixtures.mjs) and
// is the oracle for the port: every comparison here is byte for byte, and a change that makes a
// case pass without fixing the port is a regression.

// wantParse is the shape a `.want.json` has: a request on `ok`, a reason on `error`, nothing on
// `none`.
type wantParse struct {
	Kind    string     `json:"kind"`
	Format  string     `json:"format"`
	Method  string     `json:"method"`
	URL     string     `json:"url"`
	Headers [][]string `json:"headers"`
	Body    string     `json:"body"`
	Reason  string     `json:"reason"`
}

// wantExport is the shape a `.in.json` has.
type wantExport struct {
	Format  string     `json:"format"`
	Method  string     `json:"method"`
	URL     string     `json:"url"`
	Headers [][]string `json:"headers"`
	Body    string     `json:"body"`
}

// readJSON reads and parses one fixture file.
func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
}

// TestParseFixtures reads every paste in testdata/parse and compares the whole result: the kind, and
// on `ok` the format, method, URL, body and the header sequence — names, values and order together,
// so a header that was lost, renamed or moved fails here rather than in a request that quietly goes
// somewhere else.
func TestParseFixtures(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "parse", "*.cmd"))
	if err != nil {
		t.Fatalf("globbing parse fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no parse fixtures found")
	}

	counts := map[string]int{}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".cmd")
		// The input is read as bytes: the corpus contains CRLF, a BOM and $'…', and normalising any
		// of them here would test something other than the parser.
		input, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		var want wantParse
		readJSON(t, filepath.Join("testdata", "parse", name+".want.json"), &want)
		counts[want.Kind]++

		t.Run(name, func(t *testing.T) {
			got := Parse(string(input))
			switch want.Kind {
			case "none":
				if got.Kind != KindNone {
					t.Fatalf("Parse(%q) = %+v, want kind none", input, got)
				}
			case "ok":
				if got.Kind != KindOK {
					t.Fatalf("Parse(%q) = %+v, want kind ok", input, got)
				}
				if string(got.Format) != want.Format {
					t.Errorf("Parse(%q) format = %q, want %q", input, got.Format, want.Format)
				}
				assertRequest(t, got.Request, want)
			case "error":
				if got.Kind != KindError {
					t.Fatalf("Parse(%q) = %+v, want kind error", input, got)
				}
				if string(got.Reason) != want.Reason {
					t.Errorf("Parse(%q) reason = %q, want %q", input, got.Reason, want.Reason)
				}
			default:
				t.Fatalf("%s has an unknown kind %q", name, want.Kind)
			}
		})
	}
	t.Logf("parse fixtures: %d ok, %d none, %d error", counts["ok"], counts["none"], counts["error"])
}

// assertRequest compares a parsed request against the fixture, field by field.
func assertRequest(t *testing.T, got Request, want wantParse) {
	t.Helper()
	if got.Method != want.Method {
		t.Errorf("method = %q, want %q", got.Method, want.Method)
	}
	if got.URL != want.URL {
		t.Errorf("url = %q, want %q", got.URL, want.URL)
	}
	if got.Body != want.Body {
		t.Errorf("body = %q, want %q", got.Body, want.Body)
	}
	assertHeaders(t, got.Headers, want.Headers)
}

// assertHeaders compares the header sequence as a sequence: two requests with the same headers in a
// different order are not the same request.
func assertHeaders(t *testing.T, got []Header, want [][]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("headers = %s, want %s", formatHeaders(got), formatWantHeaders(want))
	}
	for i := range want {
		if len(want[i]) != 2 {
			t.Fatalf("fixture header %d is not a name/value pair: %v", i, want[i])
		}
		if got[i].Name != want[i][0] || got[i].Value != want[i][1] {
			t.Errorf("header %d = %q: %q, want %q: %q",
				i, got[i].Name, got[i].Value, want[i][0], want[i][1])
		}
	}
}

func formatHeaders(hs []Header) string {
	parts := make([]string, 0, len(hs))
	for _, h := range hs {
		parts = append(parts, h.Name+": "+h.Value)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatWantHeaders(hs [][]string) string {
	parts := make([]string, 0, len(hs))
	for _, h := range hs {
		parts = append(parts, strings.Join(h, ": "))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// TestExportFixtures renders each request in each format and compares the bytes. The expected text
// is taken as it is — no trailing newline is added or trimmed.
func TestExportFixtures(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "export", "*.in.json"))
	if err != nil {
		t.Fatalf("globbing export fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no export fixtures found")
	}

	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".in.json")
		var in wantExport
		readJSON(t, path, &in)
		want, err := os.ReadFile(filepath.Join("testdata", "export", name+".want.txt"))
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		t.Run(name, func(t *testing.T) {
			got := Export(Format(in.Format), requestFrom(in), ExportOptions{})
			if got != string(want) {
				t.Errorf("Export(%s) =\n%q\nwant\n%q", in.Format, got, string(want))
			}
		})
	}
}

// requestFrom builds the request an export fixture describes, keeping the header order it lists.
func requestFrom(in wantExport) Request {
	req := Request{Method: in.Method, URL: in.URL, Body: in.Body}
	for _, pair := range in.Headers {
		if len(pair) != 2 {
			continue
		}
		req.Headers = append(req.Headers, Header{Name: pair[0], Value: pair[1]})
	}
	return req
}

// TestRoundTrip exports every command the corpus parses and parses it back: the two halves have to
// be each other's inverse for the cases where the TS is one, which is what stops the exporter and
// the parser from drifting apart in opposite directions.
func TestRoundTrip(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "parse", "*.cmd"))
	if err != nil {
		t.Fatalf("globbing parse fixtures: %v", err)
	}

	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".cmd")
		input, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		t.Run(name, func(t *testing.T) {
			first := Parse(string(input))
			if first.Kind != KindOK {
				// A paste that is not a command has nothing to round-trip.
				return
			}

			exported := Export(first.Format, first.Request, ExportOptions{})
			second := Parse(exported)
			if second.Kind != KindOK {
				t.Fatalf("re-parsing the export of %s = %+v; export was %q", name, second, exported)
			}
			if second.Format != first.Format {
				t.Errorf("format = %q, want %q (export was %q)", second.Format, first.Format, exported)
			}
			if second.Request.Method != first.Request.Method {
				t.Errorf("method = %q, want %q (export was %q)",
					second.Request.Method, first.Request.Method, exported)
			}
			if second.Request.URL != first.Request.URL {
				t.Errorf("url = %q, want %q (export was %q)",
					second.Request.URL, first.Request.URL, exported)
			}
			if second.Request.Body != first.Request.Body {
				t.Errorf("body = %q, want %q (export was %q)",
					second.Request.Body, first.Request.Body, exported)
			}
			if len(second.Request.Headers) != len(first.Request.Headers) {
				t.Fatalf("headers = %s, want %s (export was %q)",
					formatHeaders(second.Request.Headers), formatHeaders(first.Request.Headers), exported)
			}
			for i := range first.Request.Headers {
				if second.Request.Headers[i] != first.Request.Headers[i] {
					t.Errorf("header %d = %v, want %v (export was %q)",
						i, second.Request.Headers[i], first.Request.Headers[i], exported)
				}
			}
		})
	}
}

// TestExportSubstitutesTokens pins what the export does with `{{tokens}}`: the editor keeps them so
// that the same command can be pasted back later, while a send-time export resolves them — and a
// secret leaves as the mask, because an exported command outlives the moment of sending.
func TestExportSubstitutesTokens(t *testing.T) {
	req := Request{
		Method:  "GET",
		URL:     "{{host}}/articles",
		Headers: []Header{{Name: "Authorization", Value: "Bearer {{token}}"}},
	}

	kept := Export(FormatCurl, req, ExportOptions{})
	if want := "curl -H 'Authorization: Bearer {{token}}' '{{host}}/articles'"; kept != want {
		t.Errorf("Export with no resolver = %q, want %q", kept, want)
	}

	opts := ExportOptions{Resolve: func(name string) (vars.Resolution, bool) {
		switch name {
		case "host":
			return vars.Resolution{Value: "api.example.com", Kind: vars.KindText}, true
		case "token":
			return vars.Resolution{Value: "s3cret", Kind: vars.KindSecret}, true
		}
		return vars.Resolution{}, false
	}}
	got := Export(FormatCurl, req, opts)
	want := "curl -H 'Authorization: Bearer " + vars.SecretMask + "' 'api.example.com/articles'"
	if got != want {
		t.Errorf("Export with a resolver = %q, want %q", got, want)
	}

	// keepTokens wins over the resolver: the editor writes out what the user typed.
	if got := Export(FormatCurl, req, ExportOptions{Resolve: opts.Resolve, KeepTokens: true}); got != kept {
		t.Errorf("Export with keepTokens = %q, want %q", got, kept)
	}
}

// TestPropertyOrder pins what the corpus cannot reach: the TS hands header lists and JSON bodies
// through plain JS objects, and an ordinary object enumerates a key that looks like an array index
// before the rest, in ascending numeric order. Header names and JSON keys that are numbers are rare
// but real, and reproducing the order means a parsed request and an exported command match the TS
// byte for byte rather than merely carrying the same values.
func TestPropertyOrder(t *testing.T) {
	parsed := Parse("curl -H '2: x' -H '10: y' -H '1: z' -H 'A: w' https://api.example.com/articles")
	if parsed.Kind != KindOK {
		t.Fatalf("Parse = %+v, want ok", parsed)
	}
	wantHeaders := [][]string{{"1", "z"}, {"2", "x"}, {"10", "y"}, {"A", "w"}}
	assertHeaders(t, parsed.Request.Headers, wantHeaders)

	exported := Export(FormatCurl, parsed.Request, ExportOptions{})
	wantExport := "curl -H '1: z' -H '2: x' -H '10: y' -H 'A: w' 'https://api.example.com/articles'"
	if exported != wantExport {
		t.Errorf("Export = %q, want %q", exported, wantExport)
	}

	// The same rule decides the text JSON.stringify writes for a body built from httpie fields.
	body := Parse("http POST https://api.example.com/articles 10=x 9=y")
	if body.Kind != KindOK {
		t.Fatalf("Parse = %+v, want ok", body)
	}
	if want := `{"9":"y","10":"x"}`; body.Request.Body != want {
		t.Errorf("body = %q, want %q", body.Request.Body, want)
	}
}

// TestExportCollapsesRepeatedHeaders mirrors what the TS gets from building a plain object: a name
// that appears twice keeps the position of its first occurrence and the value of its last.
func TestExportCollapsesRepeatedHeaders(t *testing.T) {
	req := Request{
		Method: "GET",
		URL:    "https://api.example.com/articles",
		Headers: []Header{
			{Name: "Accept", Value: "application/json"},
			{Name: "X-B", Value: "2"},
			{Name: "Accept", Value: "application/vnd.api+json"},
		},
	}
	want := "curl -H 'Accept: application/vnd.api+json' -H 'X-B: 2' 'https://api.example.com/articles'"
	if got := Export(FormatCurl, req, ExportOptions{}); got != want {
		t.Errorf("Export = %q, want %q", got, want)
	}
}
