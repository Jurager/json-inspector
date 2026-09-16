package draft

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
)

type preparedLine struct {
	uc   *UseCase
	auth *fakeAuth
}

func newLine(t *testing.T) preparedLine {
	t.Helper()
	uc, _, _, auth := newWithFakes()
	if err := uc.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := uc.Replace(context.Background(), domain.DraftCommandLine, Seed{
		Method: "GET",
		URL:    "https://api.example.com/a",
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	return preparedLine{uc: uc, auth: auth}
}

func (l preparedLine) state(t *testing.T) State {
	t.Helper()
	state, err := l.uc.Snapshot(context.Background(), domain.DraftCommandLine)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	return state
}

func (l preparedLine) setAuth(t *testing.T, auth domain.Auth) {
	t.Helper()
	if _, err := l.uc.SetAuth(context.Background(), domain.DraftCommandLine, auth); err != nil {
		t.Fatalf("SetAuth: %v", err)
	}
}

func rowOf(
	rows []domain.ProjectedRow,
	target domain.RowKind,
	name string,
) (domain.ProjectedRow, bool) {
	for _, row := range rows {
		if row.Target == target && row.Name == name {
			return row, true
		}
	}
	return domain.ProjectedRow{}, false
}

// The window has nowhere else to learn which list a projected row belongs to or which scheme put
// it there.
func TestProjectedRowsTravelWithTheDraft(t *testing.T) {
	line := newLine(t)
	line.auth.answer = domain.AuthOutput{
		Headers:  []domain.HeaderPair{{Name: "Authorization", Value: "Bearer abc"}},
		Query:    []domain.HeaderPair{{Name: "X-API-Key", Value: "s3cret"}},
		Editable: true,
	}
	line.setAuth(t, domain.NewAuth(domain.AuthBearer))

	state := line.state(t)
	header, ok := rowOf(state.Projected, domain.RowHeaders, "Authorization")
	if !ok {
		t.Fatalf("projected = %+v, want the header the scheme put there", state.Projected)
	}
	if header.Value != "Bearer abc" || header.From != domain.AuthBearer || !header.Editable {
		t.Errorf("projected header = %+v, want what the scheme answered", header)
	}

	param, ok := rowOf(state.Projected, domain.RowParams, "X-API-Key")
	if !ok || param.Value != "s3cret" {
		t.Errorf("projected = %+v, want the query parameter beside the header", state.Projected)
	}

	// Nothing of this is written down: the draft holds the scheme's fields, and the rows a scheme
	// projects are not among its own.
	if len(state.Draft.Headers) != 0 {
		t.Errorf("draft headers = %+v, want none of the projection among them", state.Draft.Headers)
	}
	if len(state.Draft.Params) != 0 {
		t.Errorf("draft params = %+v, want none of the projection among them", state.Draft.Params)
	}
}

// A row a person wrote under that name is the better answer, and two Authorization headers are a
// request no server reads the way the window drew it. The rule is the one sending follows, so the
// list the window draws is the list that goes out.
func TestAWrittenRowDropsTheProjection(t *testing.T) {
	line := newLine(t)
	line.auth.answer = domain.AuthOutput{
		Headers:  []domain.HeaderPair{{Name: "Authorization", Value: "Bearer abc"}},
		Query:    []domain.HeaderPair{{Name: "X-API-Key", Value: "s3cret"}},
		Editable: true,
	}
	if _, err := line.uc.Replace(context.Background(), domain.DraftCommandLine, Seed{
		Method:  "GET",
		URL:     "https://api.example.com/a?X-API-Key=hand-written",
		Headers: []domain.HeaderPair{{Name: "authorization", Value: "Token hand-written"}},
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	line.setAuth(t, domain.NewAuth(domain.AuthBearer))

	if _, ok := rowOf(line.state(t).Projected, domain.RowHeaders, "Authorization"); ok {
		t.Error("the projection survived a row the user wrote themselves")
	}
	if _, ok := rowOf(line.state(t).Projected, domain.RowParams, "X-API-Key"); ok {
		t.Error("the projection survived a parameter already in the address")
	}
}

// A row that is switched off does not go out, so it is not the better answer to anything — and
// dropping the projection over it would hide a credential that is on its way.
func TestADisabledRowDoesNotDropTheProjection(t *testing.T) {
	line := newLine(t)
	line.auth.answer = domain.AuthOutput{
		Headers:  []domain.HeaderPair{{Name: "Authorization", Value: "Bearer abc"}},
		Editable: true,
	}
	if _, err := line.uc.Replace(context.Background(), domain.DraftCommandLine, Seed{
		Method:  "GET",
		URL:     "https://api.example.com/a",
		Headers: []domain.HeaderPair{{Name: "Authorization", Value: "Token hand-written"}},
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	state := line.state(t)
	if _, err := line.uc.PatchRow(context.Background(), domain.DraftCommandLine,
		domain.RowHeaders, state.Draft.Headers[0].ID, RowPatch{Enabled: boolPtr(false)}); err != nil {
		t.Fatalf("PatchRow: %v", err)
	}
	line.setAuth(t, domain.NewAuth(domain.AuthBearer))

	if _, ok := rowOf(line.state(t).Projected, domain.RowHeaders, "Authorization"); !ok {
		t.Error("a switched-off row took the projection with it")
	}
}

// Editing a projected row is editing the field behind it: the row is what the fields come to, and
// there is nothing else to change.
func TestEditingAProjectedRowEditsTheField(t *testing.T) {
	line := newLine(t)
	line.auth.absorbInto = "token"
	line.setAuth(t, domain.NewAuth(domain.AuthBearer).With("token", "first"))

	if _, err := line.uc.PatchDerived(context.Background(), domain.DraftCommandLine,
		domain.RowHeaders, "Authorization", "second"); err != nil {
		t.Fatalf("PatchDerived: %v", err)
	}

	if got := line.state(t).Draft.Auth.Answer("token"); got != "second" {
		t.Errorf("token = %q, want the edit to have reached the field", got)
	}
}

// A scheme that cannot take the edit refuses it rather than guessing at what it meant — a Basic
// credential is a login and a password encoded together, and base64 does not come apart.
func TestASchemeThatCannotAbsorbRefuses(t *testing.T) {
	line := newLine(t)
	line.setAuth(t, domain.NewAuth(domain.AuthBasic).With("username", "user"))

	_, err := line.uc.PatchDerived(context.Background(), domain.DraftCommandLine,
		domain.RowHeaders, "Authorization", "Basic dXNlcjpwYXNz")
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("PatchDerived: %v, want the refusal the window does not offer an edit for", err)
	}
	if got := line.state(t).Draft.Auth.Answer("username"); got != "user" {
		t.Errorf("username = %q, want the refusal to have changed nothing", got)
	}
}

// Deleting a projected row deletes what put it there. A request that no longer authorizes itself is
// what the user asked for by removing the only row the authorization was.
func TestDeletingAProjectedRowRemovesTheAuthorization(t *testing.T) {
	line := newLine(t)
	line.auth.absorbInto = "token"
	line.setAuth(t, domain.NewAuth(domain.AuthBearer).With("token", "abc"))

	if _, err := line.uc.RemoveDerived(context.Background(), domain.DraftCommandLine); err != nil {
		t.Fatalf("RemoveDerived: %v", err)
	}
	auth := line.state(t).Draft.Auth
	if !auth.IsNone() {
		t.Errorf("auth = %q, want «нет»", auth.Type)
	}
	if _, ok := rowOf(line.state(t).Projected, domain.RowHeaders, "Authorization"); ok {
		t.Error("the projection outlived the authorization that made it")
	}
}

// A `{{token}}` written into a scheme that is not in use is not a variable of this request: nothing
// reads that answer, so nothing fills it in and nothing can be missing from it. The alternative —
// walking every scheme's fields — would block a send over a token the request was never going to
// carry.
func TestAnUnusedSchemesAnswersAreNotSubstituted(t *testing.T) {
	line := newLine(t)
	line.setAuth(t, domain.NewAuth(domain.AuthBasic).
		With("username", "user").With("password", "pass").
		With("token", "{{nothing-resolves-this}}"))

	for _, name := range line.state(t).Preview.Missing {
		if name == "nothing-resolves-this" {
			t.Error("a variable in a scheme that is not in use was reported missing")
		}
	}
}

// «Inherit» is not this feature's to answer — a draft cannot walk a tree — so the answer is
// handed in, and the rows it comes to are worked out the same way a request's own are. That is what
// makes a request inside a collection show the credential it inherited in its header list.
func TestAnInheritedAuthorizationIsProjectedFromWhatIsHandedIn(t *testing.T) {
	line := newLine(t)
	line.setAuth(t, domain.NewAuth(domain.AuthInherit))

	// The draft on its own knows nothing above it, and asks with «None» rather than guessing: there is
	// no scheme to project and so no row.
	state := line.state(t)
	if !line.auth.shown[len(line.auth.shown)-1].IsNone() {
		t.Errorf("the scheme was asked with %q, want «None» for a walk the draft cannot make",
			line.auth.shown[len(line.auth.shown)-1].Type)
	}
	if len(state.Projected) != 0 {
		t.Errorf("projected = %+v, want nothing from a walk the draft cannot make", state.Projected)
	}

	line.auth.answer = domain.AuthOutput{
		Headers:  []domain.HeaderPair{{Name: "Authorization", Value: "Bearer from-above"}},
		Editable: true,
	}
	inherited := domain.NewAuth(domain.AuthBearer).With("token", "from-above")
	projected, err := line.uc.Project(context.Background(), state.Draft, &inherited)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	row, ok := rowOf(projected, domain.RowHeaders, "Authorization")
	if !ok || row.Value != "Bearer from-above" {
		t.Errorf("projected = %+v, want the row the inherited answer comes to", projected)
	}
	if got := line.auth.shown[len(line.auth.shown)-1].Answer("token"); got != "from-above" {
		t.Errorf("the scheme was asked with %q, want what the level above answered", got)
	}
}
