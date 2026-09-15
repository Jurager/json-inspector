package draft

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"json-inspector/internal/domain"
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
	// Auth is the scheme a command's flags come to, which is not a header and is not in `headers`:
	// `<type>` and the answers to its fields, as the fixture writes them out.
	Auth *wantAuth `json:"auth"`
}

type wantAuth struct {
	Type   string            `json:"type"`
	Fields map[string]string `json:"fields"`
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

// TestParseFixtures reads every paste in testdata/parse and compares the whole result: the kind,
// and on `ok` the format, method, URL, body and the header sequence — names, values and order
// together, so a header that was lost, renamed or moved fails here rather than in a request that
// quietly goes somewhere else.
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
			got := ParseCommand(string(input))
			switch want.Kind {
			case "none":
				if got.Kind != KindNone {
					t.Fatalf("ParseCommand(%q) = %+v, want kind none", input, got)
				}
			case "ok":
				if got.Kind != KindOK {
					t.Fatalf("ParseCommand(%q) = %+v, want kind ok", input, got)
				}
				// The reader knows one tool, and a fixture of any other format surviving the cut
				// to curl alone is a regression in `detect` rather than a corpus that needs
				// editing.
				if want.Format != "curl" {
					t.Fatalf("%s: a %q fixture is still expected to parse", name, want.Format)
				}
				assertRequest(t, got.Seed, want)
			case "error":
				if got.Kind != KindError {
					t.Fatalf("ParseCommand(%q) = %+v, want kind error", input, got)
				}
				if string(got.Reason) != want.Reason {
					t.Errorf("ParseCommand(%q) reason = %q, want %q", input, got.Reason, want.Reason)
				}
			default:
				t.Fatalf("%s has an unknown kind %q", name, want.Kind)
			}
		})
	}
	t.Logf("parse fixtures: %d ok, %d none, %d error", counts["ok"], counts["none"], counts["error"])
}

// assertRequest compares a parsed request against the fixture, field by field.
func assertRequest(t *testing.T, got Seed, want wantParse) {
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
	assertAuth(t, got.Auth, want.Auth)
}

// assertAuth compares the scheme a command came to. A fixture that says nothing about one is a
// command with no credential in it, and a parse that invented one fails here.
func assertAuth(t *testing.T, got *domain.Auth, want *wantAuth) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("auth = %+v, want %+v", got, want)
	}
	if got == nil {
		return
	}
	if string(got.Type) != want.Type {
		t.Errorf("auth type = %q, want %q", got.Type, want.Type)
	}
	for key, value := range want.Fields {
		if got.Answer(key) != value {
			t.Errorf("auth %s = %q, want %q", key, got.Answer(key), value)
		}
	}
}

// assertHeaders compares the header sequence as a sequence: two requests with the same headers
// in a different order are not the same request.
func assertHeaders(t *testing.T, got []domain.HeaderPair, want [][]string) {
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

func formatHeaders(hs []domain.HeaderPair) string {
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

// TestExportFixtures renders each request in each format and compares the bytes. The expected
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
			got := RenderCommand(CommandFormat(in.Format), requestFrom(in))
			if got != string(want) {
				t.Errorf("RenderCommand(%s) =\n%q\nwant\n%q", in.Format, got, string(want))
			}
		})
	}
}

// requestFrom builds the request an export fixture describes, keeping the header order it lists.
func requestFrom(in wantExport) Seed {
	req := Seed{Method: in.Method, URL: in.URL, Body: in.Body}
	for _, pair := range in.Headers {
		if len(pair) != 2 {
			continue
		}
		req.Headers = append(req.Headers, domain.HeaderPair{Name: pair[0], Value: pair[1]})
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
			first := ParseCommand(string(input))
			if first.Kind != KindOK {
				// A paste that is not a command has nothing to round-trip.
				return
			}

			exported := RenderCommand(FormatCurl, first.Seed)
			second := ParseCommand(exported)
			if second.Kind != KindOK {
				t.Fatalf("re-parsing the export of %s = %+v; export was %q", name, second, exported)
			}
			if second.Seed.Method != first.Seed.Method {
				t.Errorf("method = %q, want %q (export was %q)",
					second.Seed.Method, first.Seed.Method, exported)
			}
			if second.Seed.URL != first.Seed.URL {
				t.Errorf("url = %q, want %q (export was %q)",
					second.Seed.URL, first.Seed.URL, exported)
			}
			if second.Seed.Body != first.Seed.Body {
				t.Errorf("body = %q, want %q (export was %q)",
					second.Seed.Body, first.Seed.Body, exported)
			}
			if len(second.Seed.Headers) != len(first.Seed.Headers) {
				t.Fatalf("headers = %s, want %s (export was %q)",
					formatHeaders(second.Seed.Headers), formatHeaders(first.Seed.Headers), exported)
			}
			for i := range first.Seed.Headers {
				if second.Seed.Headers[i] != first.Seed.Headers[i] {
					t.Errorf("header %d = %v, want %v (export was %q)",
						i, second.Seed.Headers[i], first.Seed.Headers[i], exported)
				}
			}
		})
	}
}

// TestWrittenOrder pins what the corpus cannot reach: header lists and JSON bodies keep the order
// they were written in.
//
// This port used to reproduce a JavaScript quirk here instead — an ordinary JS object enumerates a
// key that looks like an array index before the rest, in ascending numeric order, so `2: x` and
// `10: y` came out before `1: z`. That was fidelity to a TypeScript implementation that no longer
// exists, and it was not fidelity to anything else: a request's headers go out in the order
// somebody wrote them in. The corpus says nothing about the difference — it has no case with a
// numeric header name — so this test is the only place the rule is written down.
func TestWrittenOrder(t *testing.T) {
	parsed := ParseCommand("curl -H '2: x' -H '10: y' -H '1: z' -H 'A: w' " +
		"https://api.example.com/articles")
	if parsed.Kind != KindOK {
		t.Fatalf("Parse = %+v, want ok", parsed)
	}
	wantHeaders := [][]string{{"2", "x"}, {"10", "y"}, {"1", "z"}, {"A", "w"}}
	assertHeaders(t, parsed.Seed.Headers, wantHeaders)

	exported := RenderCommand(FormatCurl, parsed.Seed)
	wantExport := "curl -H '2: x' -H '10: y' -H '1: z' -H 'A: w' 'https://api.example.com/articles'"
	if exported != wantExport {
		t.Errorf("Export = %q, want %q", exported, wantExport)
	}

	// The rule survives the round trip: what the reader put in that order is what the command
	// writes back out, so a copy of a copy is the same request.
	again := ParseCommand(exported)
	if again.Kind != KindOK {
		t.Fatalf("ParseCommand(%q) = %+v, want ok", exported, again)
	}
	assertHeaders(t, again.Seed.Headers, wantHeaders)
}

// TestExportCollapsesRepeatedHeaders mirrors what the TS gets from building a plain object: a name
// that appears twice keeps the position of its first occurrence and the value of its last.
func TestExportCollapsesRepeatedHeaders(t *testing.T) {
	req := Seed{
		Method: "GET",
		URL:    "https://api.example.com/articles",
		Headers: []domain.HeaderPair{
			{Name: "Accept", Value: "application/json"},
			{Name: "X-B", Value: "2"},
			{Name: "Accept", Value: "application/vnd.api+json"},
		},
	}
	want := "curl -H 'Accept: application/vnd.api+json' -H 'X-B: 2' " +
		"'https://api.example.com/articles'"
	if got := RenderCommand(FormatCurl, req); got != want {
		t.Errorf("Export = %q, want %q", got, want)
	}
}
