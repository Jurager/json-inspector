package command

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzParse feeds arbitrary pastes to the parser. It asserts only what has to hold whatever the
// input is — no panic, and an `ok` result that carries a method and a URL — because everything else,
// down to the byte, is pinned by the corpus in testdata/parse. A fuzz-only assertion about, say, the
// header list would be a second, weaker specification of the same thing.
//
// The second half is what the fuzzer earned its keep on: `fetch("")` and `curl -X "" URL` used to
// come back as `ok` with an empty field, exactly as the TypeScript does. Parse now answers both
// instead — see TestDeliberateDivergences.
func FuzzParse(f *testing.F) {
	paths, err := filepath.Glob(filepath.Join("testdata", "parse", "*.cmd"))
	if err != nil {
		f.Fatalf("globbing parse fixtures: %v", err)
	}
	if len(paths) == 0 {
		f.Fatal("no parse fixtures to seed the fuzzer with")
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatalf("reading %s: %v", path, err)
		}
		f.Add(string(data))
	}

	f.Fuzz(func(t *testing.T, text string) {
		got := Parse(text)
		if got.Kind != KindOK {
			return
		}
		if got.Request.Method == "" {
			t.Fatalf("Parse(%q) = ok with an empty method", text)
		}
		if got.Request.URL == "" {
			t.Fatalf("Parse(%q) = ok with an empty URL", text)
		}
	})
}
