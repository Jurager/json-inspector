package command

import "testing"

// TestDeliberateDivergences pins the two places where this port knowingly differs from the
// TypeScript it was dumped from.
//
// Both are inputs the TS answers with `ok` while leaving the caller unable to build a request:
// `fetch("")` yields an empty URL, and `curl -X "" URL` an empty method. Neither appears in the
// corpus, so the port stays byte-identical everywhere the corpus looks; here it is stricter on
// purpose, because an `ok` result is what makes the frontend overwrite the user's draft.
//
// If a future change makes either of these parse as `ok` again, that is a regression, not a
// simplification — delete nothing here without reading the reason above.
func TestDeliberateDivergences(t *testing.T) {
	t.Run("an empty URL is reported, not accepted", func(t *testing.T) {
		got := Parse(`fetch("")`)
		if got.Kind != KindError || got.Reason != ReasonNoURL {
			t.Errorf(`Parse(fetch("")) = %+v, want kind error with reason %q`, got, ReasonNoURL)
		}
	})

	t.Run("an explicitly empty method falls back to GET", func(t *testing.T) {
		got := Parse(`curl -X "" https://api.example.com/articles`)
		if got.Kind != KindOK {
			t.Fatalf(`Parse(curl -X "") = %+v, want kind ok`, got)
		}
		if got.Request.Method != "GET" {
			t.Errorf("method = %q, want GET", got.Request.Method)
		}
		if got.Request.URL != "https://api.example.com/articles" {
			t.Errorf("url = %q, want the URL that was written", got.Request.URL)
		}
	})
}
