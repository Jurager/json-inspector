package draft

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
	"json-inspector/internal/vars"
)

type fakeStore struct {
	saved    domain.Draft
	has      bool
	failSave error
}

func (f *fakeStore) Draft(_ context.Context, id domain.DraftID) (domain.Draft, error) {
	if !f.has {
		return domain.Draft{}, domain.ErrNotFound
	}
	return f.saved, nil
}

func (f *fakeStore) SaveDraft(_ context.Context, draft domain.Draft) error {
	if f.failSave != nil {
		return f.failSave
	}
	f.saved, f.has = draft, true
	return nil
}

// fakeVars is the environment feature as the draft sees it: values for the names it knows, nothing
// for the rest, and a secret that leaves as its mask.
type fakeVars struct {
	values map[string]string
	secret map[string]bool
}

func (f fakeVars) resolver() vars.Resolver {
	return func(name string) (domain.Resolution, bool) {
		value, ok := f.values[name]
		if !ok {
			return domain.Resolution{}, false
		}
		kind := domain.VariableText
		if f.secret[name] {
			kind = domain.VariableSecret
		}
		return domain.Resolution{Value: value, Kind: kind, Source: "env", HasValue: value != ""}, true
	}
}

func (f fakeVars) Missing(_ context.Context, texts []string) ([]string, error) {
	_, got := missingIn(texts, f.resolver())
	return got, nil
}

func (f fakeVars) SubstituteTexts(_ context.Context, texts []string, mask bool) ([]string, error) {
	out := make([]string, len(texts))
	for i, text := range texts {
		if mask {
			out[i] = vars.SubstituteMasked(text, f.resolver())
			continue
		}
		out[i] = vars.Substitute(text, f.resolver())
	}
	return out, nil
}

// missingIn is the merge the environment feature does, kept here so the fake answers the same way.
func missingIn(texts []string, resolve vars.Resolver) (map[string]bool, []string) {
	seen := map[string]bool{}
	out := []string{}
	for _, text := range texts {
		for _, name := range vars.Missing(text, resolve) {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return seen, out
}

func newUseCase() (*UseCase, *fakeStore) {
	store := &fakeStore{}
	variables := fakeVars{
		values: map[string]string{"host": "api.example.com", "token": "abc123", "nothing": ""},
		secret: map[string]bool{"token": true},
	}
	return NewUseCase(store, variables, platform.NewIDGen()), store
}

func loaded(t *testing.T) (*UseCase, *fakeStore) {
	t.Helper()
	uc, store := newUseCase()
	if err := uc.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return uc, store
}

func TestLoadStartsOnTheAcceptHeader(t *testing.T) {
	uc, _ := loaded(t)
	state, err := uc.Snapshot(context.Background(), domain.DraftCommandLine)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	draft := state.Draft
	if draft.Method != "GET" || len(draft.Headers) != 1 || draft.Headers[0].Name != "Accept" {
		t.Fatalf("a fresh draft = %+v, want a GET with an Accept header", draft)
	}
	if draft.Headers[0].ID == "" {
		t.Error("the seeded row has no id, so nothing can address it")
	}
}

func TestLoadReadsWhatWasStored(t *testing.T) {
	uc, store := newUseCase()
	store.has, store.saved = true, domain.Draft{
		ID: domain.DraftCommandLine, Revision: 7, Method: "POST", URL: "/a?x=1",
	}

	if err := uc.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	draft, _ := uc.Snapshot(context.Background(), domain.DraftCommandLine)
	if draft.Draft.Method != "POST" || draft.Draft.Revision != 7 {
		t.Errorf("draft = %+v, want the stored one", draft)
	}
}

// The buffer: what the user types reaches this side as a text, and the rows follow it. An id is
// kept across an edit, because an open editor holds one.
func TestSetTextURLDrivesTheRows(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	result, err := uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldURL, Text: "/a?x=1&y=2", Rev: 4})
	if err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if result.Rev != 4 {
		t.Errorf("rev = %d, want the window's own back", result.Rev)
	}
	if len(result.Draft.Params) != 2 {
		t.Fatalf("params = %+v, want two", result.Draft.Params)
	}
	first, second := result.Draft.Params[0].ID, result.Draft.Params[1].ID

	// The user edits the text: y becomes z, and x stays where it was.
	result, err = uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldURL, Text: "/a?x=1&z=3", Rev: 5})
	if err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if len(result.Draft.Params) != 2 {
		t.Fatalf("params = %+v, want still two", result.Draft.Params)
	}
	if result.Draft.Params[0].ID != first {
		t.Errorf("row x = %q, want the id it had", result.Draft.Params[0].ID)
	}
	if result.Draft.Params[1].ID == second {
		t.Error("row z kept the id of the row it replaced")
	}
	if result.Draft.URL != "/a?x=1&z=3" {
		t.Errorf("url = %q, want the text as typed", result.Draft.URL)
	}
}

// A parked row survives a keystroke in the URL: it is not in the text, and that is the point of it.
func TestSetTextKeepsDisabledRows(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	flushed, err := uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldURL, Text: "/a?x=1&y=2"})
	if err != nil {
		t.Fatalf("SetText: %v", err)
	}
	off := flushed.Draft.Params[1].ID

	parked, err := uc.PatchRow(ctx, domain.DraftCommandLine, domain.RowParams, off, RowPatch{Enabled: boolPtr(false)})
	if err != nil {
		t.Fatalf("PatchRow: %v", err)
	}
	if parked.Draft.URL != "/a?x=1" {
		t.Errorf("url = %q, want the switched-off row out of it", parked.Draft.URL)
	}

	flushed, err = uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldURL, Text: "/a?x=1&w=3"})
	if err != nil {
		t.Fatalf("SetText: %v", err)
	}
	var kept *domain.Row
	for i, row := range flushed.Draft.Params {
		if row.ID == off {
			kept = &flushed.Draft.Params[i]
		}
	}
	if kept == nil || kept.Enabled {
		t.Errorf("params = %+v, want the parked row kept and still off", flushed.Draft.Params)
	}
}

// A row edit writes the list back into the URL, and the URL keeps the shape this app's URLs have.
func TestPatchRowWritesTheURL(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	flushed, _ := uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldURL, Text: "/a?include=x&page[number]=2"})
	params := flushed.Draft.Params

	result, err := uc.PatchRow(ctx, domain.DraftCommandLine, domain.RowParams, params[0].ID, RowPatch{Value: strPtr("author,comments")})
	if err != nil {
		t.Fatalf("PatchRow: %v", err)
	}
	if result.Draft.URL != "/a?include=author,comments&page[number]=2" {
		t.Errorf("url = %q, want the comma and the brackets left as typed", result.Draft.URL)
	}

	result, err = uc.RemoveRow(ctx, domain.DraftCommandLine, domain.RowParams, params[1].ID)
	if err != nil {
		t.Fatalf("RemoveRow: %v", err)
	}
	if result.Draft.URL != "/a?include=author,comments" {
		t.Errorf("url = %q, want the removed row gone from it", result.Draft.URL)
	}
}

func TestAddRowAddressesWhatItAdds(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	result, err := uc.AddRow(ctx, domain.DraftCommandLine, domain.RowCookies)
	if err != nil {
		t.Fatalf("AddRow: %v", err)
	}
	if len(result.Draft.Cookies) != 1 || result.Draft.Cookies[0].ID == "" {
		t.Fatalf("cookies = %+v, want one with an id", result.Draft.Cookies)
	}
	if result.Draft.Cookies[0].Path != "/" {
		t.Errorf("path = %q, want the default", result.Draft.Cookies[0].Path)
	}

	if _, err := uc.AddRow(ctx, domain.DraftCommandLine, domain.RowKind("headers2")); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("adding to a list that does not exist = %v, want ErrNotAllowed", err)
	}
}

// A patch for a row that is gone is an error, and the draft is left as it was: rolling back is what
// keeps memory and the database saying the same thing.
func TestPatchOnAMissingRowChangesNothing(t *testing.T) {
	uc, store := loaded(t)
	ctx := context.Background()

	before, _ := uc.Snapshot(ctx, domain.DraftCommandLine)
	if _, err := uc.PatchRow(ctx, domain.DraftCommandLine, domain.RowParams, "nope", RowPatch{Value: strPtr("x")}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("patching a row that is not there = %v, want ErrNotFound", err)
	}
	after, _ := uc.Snapshot(ctx, domain.DraftCommandLine)
	if after.Draft.Revision != before.Draft.Revision || store.saved.Revision != before.Draft.Revision {
		t.Errorf("revision moved to %d (stored %d), want it to stay at %d", after.Draft.Revision,
			store.saved.Revision, before.Draft.Revision)
	}
}

func TestASaveThatFailsIsRolledBack(t *testing.T) {
	uc, store := loaded(t)
	ctx := context.Background()

	before, _ := uc.Snapshot(ctx, domain.DraftCommandLine)
	store.failSave = errors.New("диск полон")

	if _, err := uc.SetMethod(ctx, domain.DraftCommandLine, "POST"); err == nil {
		t.Fatal("SetMethod succeeded although the draft could not be saved")
	}
	after, _ := uc.Snapshot(ctx, domain.DraftCommandLine)
	if after.Draft.Method != before.Draft.Method {
		t.Errorf("method = %q in memory, want it rolled back to %q", after.Draft.Method, before.Draft.Method)
	}
}

// Replace is what "открыть в запросе" and a pasted command both do: the draft becomes that request,
// and the jar comes from the record when there is one and from the header when there is not.
func TestReplaceTakesTheWholeRequest(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	result, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method:  "POST",
		URL:     "/a?x=1",
		Headers: []domain.HeaderPair{{Name: "Accept", Value: "application/json"}, {Name: "Cookie", Value: "a=1; b=2"}},
		Body:    `{"a":1}`,
	})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if result.Draft.Method != "POST" || result.Draft.Body != `{"a":1}` || len(result.Draft.Params) != 1 {
		t.Errorf("draft = %+v, want the seed's request", result.Draft)
	}
	if len(result.Draft.Headers) != 2 {
		t.Errorf("headers = %+v, want both kept — one of them is the Cookie the jar comes from", result.Draft.Headers)
	}
	if len(result.Draft.Cookies) != 2 || result.Draft.Cookies[0].Name != "a" {
		t.Errorf("cookies = %+v, want the header read into the jar", result.Draft.Cookies)
	}
	if result.Draft.Auth.Type != domain.AuthNone {
		t.Errorf("auth = %+v, want it reset", result.Draft.Auth)
	}

	// A record brings its own jar, and then the header is not what the jar is made of.
	result, err = uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method:  "GET",
		URL:     "/b",
		Headers: []domain.HeaderPair{{Name: "Cookie", Value: "ignored=1"}},
		Cookies: []domain.CookieRow{{Name: "kept", Value: "2", Domain: "example.com"}},
	})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if len(result.Draft.Cookies) != 1 || result.Draft.Cookies[0].Name != "kept" {
		t.Errorf("cookies = %+v, want the record's jar", result.Draft.Cookies)
	}
}

// What goes out, and what is kept beside it: the same request twice, once with the values and once
// with a secret left as its mask.
func TestPreparedFillsAndMasks(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	if _, err := uc.Replace(ctx, domain.DraftCommandLine, Seed{
		Method: "GET",
		URL:    "https://{{host}}/a?x=1",
		Headers: []domain.HeaderPair{
			{Name: "Authorization", Value: "Bearer {{token}}"},
			{Name: "X-Off", Value: "{{nothing}}"},
		},
		Body:    "body {{token}}",
		Cookies: []domain.CookieRow{{Name: "s", Value: "{{token}}"}},
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	prepared, err := uc.Prepared(ctx, domain.DraftCommandLine, nil)
	if err != nil {
		t.Fatalf("Prepared: %v", err)
	}

	if prepared.URL != "https://api.example.com/a?x=1" {
		t.Errorf("url = %q, want the variables filled in", prepared.URL)
	}
	if prepared.Body != "body abc123" {
		t.Errorf("body = %q", prepared.Body)
	}
	// The jar is what carries the cookies, and the header it becomes is built here.
	if len(prepared.Headers) != 3 {
		t.Fatalf("headers = %+v, want the two and the Cookie the jar became", prepared.Headers)
	}
	if prepared.Headers[0].Value != "Bearer abc123" {
		t.Errorf("authorization = %q, want the secret's value on the way out", prepared.Headers[0].Value)
	}
	if prepared.Headers[2].Name != "Cookie" || prepared.Headers[2].Value != "s=abc123" {
		t.Errorf("cookie header = %+v, want the jar in it", prepared.Headers[2])
	}

	if prepared.MaskedURL != "https://api.example.com/a?x=1" {
		t.Errorf("masked url = %q", prepared.MaskedURL)
	}
	if prepared.MaskedBody != "body "+vars.SecretMask {
		t.Errorf("masked body = %q, want the secret replaced", prepared.MaskedBody)
	}
	if prepared.MaskedHeaders[0].Value != "Bearer "+vars.SecretMask {
		t.Errorf("masked authorization = %q", prepared.MaskedHeaders[0].Value)
	}
	if prepared.MaskedHeaders[2].Value != "s="+vars.SecretMask {
		t.Errorf("masked cookie header = %q", prepared.MaskedHeaders[2].Value)
	}
	// The jar itself travels as it was typed: it is the draft's, and a record hands it back.
	if len(prepared.Cookies) != 1 || prepared.Cookies[0].Value != "{{token}}" {
		t.Errorf("cookies = %+v, want the rows with their tokens", prepared.Cookies)
	}
}

// The preview is the other question the command line asks: what is missing, and can this go out.
func TestPreview(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	result, err := uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldURL, Text: "https://{{host}}/a"})
	if err != nil {
		t.Fatalf("SetText: %v", err)
	}
	if len(result.Preview.Missing) != 0 {
		t.Errorf("preview = %+v, want nothing missing", result.Preview)
	}

	result, _ = uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldURL, Text: "https://{{nope}}/a"})
	if len(result.Preview.Missing) != 1 || result.Preview.Missing[0] != "nope" {
		t.Errorf("missing = %v, want nope", result.Preview.Missing)
	}

	// A header and the body are asked about too, and the same name twice is one thing missing.
	result, _ = uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldBody, Text: "{{nope}} and {{body}}"})
	if len(result.Preview.Missing) != 2 {
		t.Errorf("missing = %v, want nope once and body", result.Preview.Missing)
	}

	// A name that resolves to an empty value is known: an empty value is a value. The body is
	// cleared first, because a token in it is missing too — the warning is about the whole request.
	if _, err := uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldBody, Text: ""}); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	result, _ = uc.SetText(ctx, domain.DraftCommandLine, TextInput{Field: FieldURL, Text: "https://x/{{nothing}}"})
	if len(result.Preview.Missing) != 0 {
		t.Errorf("missing = %v, want nothing — the variable is there and empty", result.Preview.Missing)
	}
}

// A request that is not the draft — a followed link — is filled in without the draft noticing.
func TestPrepareLeavesTheDraftAlone(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	before, _ := uc.Snapshot(ctx, domain.DraftCommandLine)
	prepared, err := uc.Prepare(ctx, Seed{Method: "GET", URL: "https://{{host}}/follow"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prepared.URL != "https://api.example.com/follow" {
		t.Errorf("url = %q", prepared.URL)
	}
	after, _ := uc.Snapshot(ctx, domain.DraftCommandLine)
	if after.Draft.URL != before.Draft.URL || after.Draft.Revision != before.Draft.Revision {
		t.Errorf("the draft moved: %+v → %+v", before.Draft, after.Draft)
	}
}

// A saved request is edited as a draft of its own, keyed by the node it came from: the command
// line's draft is a different request, and switching between the two must not touch either.
func TestOpenKeepsADraftPerNode(t *testing.T) {
	uc, store := loaded(t)
	ctx := context.Background()

	node := domain.Draft{
		ID:      "node-1",
		Method:  "PATCH",
		URL:     "https://api.example.com/users/1",
		Params:  []domain.Row{{Name: "page", Value: "2", Enabled: true}},
		Headers: []domain.Row{{Name: "Accept", Value: "application/vnd.api+json", Enabled: true}},
		Body:    `{"data": 1}`,
		Cookies: []domain.CookieRow{{Name: "s", Value: "1"}},
	}
	state, err := uc.Open(ctx, node)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if state.Draft.Method != "PATCH" || state.Draft.Revision != 0 {
		t.Errorf("opened draft = %+v, want the node's request and no revision of its own", state.Draft)
	}
	// Rows arrive from the database without ids often enough that the draft has to give them some:
	// the window addresses a row by id, and a row that has none cannot be edited.
	for _, row := range state.Draft.Params {
		if row.ID == "" {
			t.Error("a parameter came out of Open without an id")
		}
	}
	if state.Draft.Cookies[0].ID == "" || state.Draft.Cookies[0].Path != "/" {
		t.Errorf("cookie = %+v, want an id and the default path", state.Draft.Cookies[0])
	}

	if _, err := uc.SetMethod(ctx, "node-1", "POST"); err != nil {
		t.Fatalf("SetMethod: %v", err)
	}
	nodeState, err := uc.Snapshot(ctx, "node-1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if nodeState.Draft.Method != "POST" || nodeState.Draft.Revision != 1 {
		t.Errorf("node draft = %+v, want the edit counted", nodeState.Draft)
	}

	// The command line was not part of any of it: its method is what it was, and nothing about the
	// node's draft reached the database.
	commandLine, err := uc.Snapshot(ctx, domain.DraftCommandLine)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if commandLine.Draft.Method != "GET" || commandLine.Draft.Revision != 0 {
		t.Errorf("the command line's draft moved: %+v", commandLine.Draft)
	}
	if store.saved.Method == "POST" {
		t.Error("a node's draft was written to the database, where only the command line's lives")
	}
}

func TestOpenReplacesThePreviousCard(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	if _, err := uc.Open(ctx, domain.Draft{ID: "node-1", Method: "GET", URL: "/a"}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := uc.SetMethod(ctx, "node-1", "POST"); err != nil {
		t.Fatalf("SetMethod: %v", err)
	}
	if _, err := uc.Open(ctx, domain.Draft{ID: "node-2", Method: "GET", URL: "/b"}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	// One card is open at a time, so the draft of the one before it is dropped rather than kept:
	// memory nobody is editing is memory nobody will free.
	if _, err := uc.Snapshot(ctx, "node-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the previous card's draft = %v, want it gone", err)
	}
	if _, err := uc.Snapshot(ctx, "node-2"); err != nil {
		t.Errorf("the open card's draft: %v", err)
	}
	if _, err := uc.Snapshot(ctx, domain.DraftCommandLine); err != nil {
		t.Errorf("the command line's draft: %v, want it untouched by a card", err)
	}
}

// Opening is what a save starts over from: the draft is rebuilt from the saved request, so nothing
// counts as unsaved any more.
func TestOpenResetsTheRevision(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	if _, err := uc.Open(ctx, domain.Draft{ID: "node-1", Method: "GET", URL: "/a"}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := uc.SetMethod(ctx, "node-1", "POST"); err != nil {
		t.Fatalf("SetMethod: %v", err)
	}

	saved, err := uc.Current(ctx, "node-1")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if _, err := uc.Open(ctx, saved); err != nil {
		t.Fatalf("Open: %v", err)
	}
	state, _ := uc.Snapshot(ctx, "node-1")
	if state.Draft.Revision != 0 {
		t.Errorf("revision = %d after opening, want the count to start over", state.Draft.Revision)
	}
}

// A request that came from a file has an address and maybe no rows of its own: the rows follow the
// address here exactly as they do when it is typed, and a row that was parked stays parked.
func TestOpenDerivesRowsFromTheAddress(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	state, err := uc.Open(ctx, domain.Draft{
		ID:      "node-1",
		Method:  "GET",
		URL:     "https://api.example.com/users?include=author&page[size]=25",
		Headers: []domain.Row{{Name: "Accept", Value: "application/json", Enabled: true}},
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(state.Draft.Params) != 2 {
		t.Fatalf("params = %+v, want the address's two", state.Draft.Params)
	}
	if state.Draft.Params[0].Name != "include" || state.Draft.Params[1].Name != "page[size]" {
		t.Errorf("params = %+v, want them in the order the address writes them", state.Draft.Params)
	}
	for _, row := range state.Draft.Params {
		if row.ID == "" {
			t.Error("a derived row has no id, so nothing can address it")
		}
	}

	// A row the file kept beside the address — switched off, so the address does not mention it —
	// is not thrown away by opening.
	state, err = uc.Open(ctx, domain.Draft{
		ID:     "node-2",
		Method: "GET",
		URL:    "https://api.example.com/users?page=1",
		Params: []domain.Row{
			{ID: "parked", Name: "filter[state]", Value: "active", Enabled: false},
			{ID: "kept", Name: "page", Value: "1", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(state.Draft.Params) != 2 || state.Draft.Params[1].ID != "parked" {
		t.Fatalf("params = %+v, want the address's row and the parked one", state.Draft.Params)
	}
	if state.Draft.Params[0].ID != "kept" {
		t.Errorf("params = %+v, want the row that is in the address to keep its id", state.Draft.Params)
	}
}

func TestADraftNobodyOpenedIsNotFound(t *testing.T) {
	uc, _ := loaded(t)
	ctx := context.Background()

	if _, err := uc.Snapshot(ctx, "node-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a draft that was never opened = %v, want ErrNotFound", err)
	}
	if _, err := uc.SetMethod(ctx, "node-1", "POST"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("editing a draft that was never opened = %v, want ErrNotFound", err)
	}
	if _, err := uc.Open(ctx, domain.Draft{Method: "GET"}); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("opening a draft without an id = %v, want ErrNotAllowed", err)
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
