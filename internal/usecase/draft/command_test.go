package draft

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"json-inspector/internal/domain"
)

// TestEveryOKResultIsUsable pins the promise an `ok` result makes: a request can be built from it,
// which is what lets the window overwrite the draft with one. An `ok` carrying an empty field
// breaks that promise, and neither case is in the corpus.
//
// One of the two was a real divergence from the TypeScript this was dumped from: `curl -X "" URL`
// came back `ok` with an empty method, which renders an empty chip. If a future change makes
// either of these read as `ok` with an empty field again, that is a regression, not a
// simplification.
func TestEveryOKResultIsUsable(t *testing.T) {
	t.Run("a command with no URL is not ok", func(t *testing.T) {
		for _, text := range []string{`curl ''`, `curl`, `curl --url '' -X POST`} {
			got := ParseCommand(text)
			if got.Kind != KindError || got.Reason != ReasonNoURL {
				t.Errorf("ParseCommand(%q) = %+v, want kind error with reason %q",
					text, got, ReasonNoURL)
			}
		}
	})

	t.Run("an explicitly empty method falls back to GET", func(t *testing.T) {
		got := ParseCommand(`curl -X "" https://api.example.com/articles`)
		if got.Kind != KindOK {
			t.Fatalf(`ParseCommand(curl -X "") = %+v, want kind ok`, got)
		}
		if got.Seed.Method != "GET" {
			t.Errorf("method = %q, want GET", got.Seed.Method)
		}
		if got.Seed.URL != "https://api.example.com/articles" {
			t.Errorf("url = %q, want the URL that was written", got.Seed.URL)
		}
	})
}

// A paste that is not a command is not a failure, and it is not a draft either: the window has to
// be able to put the text in the field, which it cannot do if this side has already replaced the
// draft with something. Most pastes into an address field are exactly this.
func TestPastingSomethingElseLeavesTheDraftAlone(t *testing.T) {
	uc, _ := newUseCase()
	for _, text := range []string{"https://api.example.com/articles", "просто текст", ""} {
		pasted, err := uc.PasteCommand(context.Background(), domain.DraftCommandLine, text)
		if err != nil {
			t.Fatalf("PasteCommand(%q): %v", text, err)
		}
		if pasted.Reading.Kind != KindNone {
			t.Errorf("PasteCommand(%q) = %q, want none", text, pasted.Reading.Kind)
		}
		if pasted.State != nil {
			t.Errorf("PasteCommand(%q) replaced the draft with %+v", text, pasted.State)
		}
	}
}

// A command is a whole request rather than an edit to one: it goes over the draft, and the reading
// comes back so the window can say so.
func TestPastingACommandReplacesTheDraft(t *testing.T) {
	uc, _ := newUseCase()
	pasted, err := uc.PasteCommand(context.Background(), domain.DraftCommandLine,
		`curl -X POST 'https://api.example.com/articles' -H 'X-A: 1' --data-raw '{"a":1}'`)
	if err != nil {
		t.Fatalf("PasteCommand: %v", err)
	}
	if pasted.Reading.Kind != KindOK {
		t.Fatalf("reading = %+v, want a POST read as curl", pasted.Reading)
	}
	if pasted.State == nil {
		t.Fatal("a command that was read left the draft as it was")
	}
	if pasted.State.Draft.Method != "POST" ||
		pasted.State.Draft.URL != "https://api.example.com/articles" {
		t.Errorf("draft = %s %s, want the command that was pasted",
			pasted.State.Draft.Method, pasted.State.Draft.URL)
	}
	if pasted.State.Draft.Body != `{"a":1}` {
		t.Errorf("body = %q, want the body the command carried", pasted.State.Draft.Body)
	}
}

// A credential pasted as a header reaches the Auth chip. Every tool that copies a request out of a
// browser writes it that way — devtools writes `-H 'Authorization: …'` and never `-u` — so a header
// left as a header would mean the chip stayed on «Нет» for the shape of command people actually
// paste, and the fields behind it unfilled.
func TestAPastedCredentialReachesTheChip(t *testing.T) {
	for _, tc := range []struct {
		what string
		text string
		want domain.Auth
	}{
		{
			what: "basic, whose value is the two answers encoded together",
			text: `curl -H 'Authorization: Basic MTIzOjMyMQ==' https://api.example.com/articles`,
			want: domain.NewAuth(domain.AuthBasic).With("username", "123").With("password", "321"),
		},
		{
			what: "bearer, and case does not matter",
			text: `curl -H 'authorization: bearer written' https://api.example.com/articles`,
			want: domain.NewAuth(domain.AuthBearer).With("token", "written"),
		},
	} {
		t.Run(tc.what, func(t *testing.T) {
			got := ParseCommand(tc.text)
			if got.Kind != KindOK {
				t.Fatalf("ParseCommand = %+v, want ok", got)
			}
			if got.Seed.Auth == nil {
				t.Fatal("auth = nil, want the scheme the header named")
			}
			if got.Seed.Auth.Type != tc.want.Type {
				t.Errorf("auth type = %q, want %q", got.Seed.Auth.Type, tc.want.Type)
			}
			for key, value := range tc.want.Fields {
				if got.Seed.Auth.Answer(key) != value {
					t.Errorf("auth %s = %q, want %q", key, got.Seed.Auth.Answer(key), value)
				}
			}
			// The row goes with it: left beside the scheme it would win over the chip, and editing
			// the chip would change nothing that goes out.
			for _, header := range got.Seed.Headers {
				if strings.EqualFold(header.Name, "Authorization") {
					t.Errorf("headers still carry %q beside the scheme", header.Name)
				}
			}
		})
	}
}

// FuzzParse feeds arbitrary pastes to the reader. It asserts only what has to hold whatever the
// input is — no panic, and an `ok` result that carries a method and a URL — because everything
// else, down to the byte, is pinned by the corpus in testdata/parse. A fuzz-only assertion about,
// say, the header list would be a second, weaker specification of the same thing.
//
// The second half is what the fuzzer earned its keep on: `curl -X "" URL` used to come back as
// `ok` with an empty method, exactly as the TypeScript does. The reading answers it instead — see
// TestEveryOKResultIsUsable.
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
		got := ParseCommand(text)
		if got.Kind != KindOK {
			return
		}
		if got.Seed.Method == "" {
			t.Fatalf("ParseCommand(%q) = ok with an empty method", text)
		}
		if got.Seed.URL == "" {
			t.Fatalf("ParseCommand(%q) = ok with an empty URL", text)
		}
	})
}
