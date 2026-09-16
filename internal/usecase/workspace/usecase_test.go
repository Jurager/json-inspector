package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// fakeStore is the switcher's two tables without a database: the workspaces in their order, and the
// pointer that names the one on screen. It keeps the rules the SQL keeps — the default space exists
// from the first launch, Personal is read back out of the id rather than written down, and a
// pointer at a row that is gone answers with the default — so the tests below are about the use
// case.
type fakeStore struct {
	workspaces []domain.Workspace
	active     string
	// deleted is every id the use case asked to remove, so a test can say a refusal never got here.
	deleted []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{workspaces: []domain.Workspace{
		domain.NewWorkspace(domain.WorkspacePersonalID, "", domain.WorkspacePersonal, "blue", 0),
	}}
}

func (f *fakeStore) has(id string) bool {
	for _, w := range f.workspaces {
		if w.ID == id {
			return true
		}
	}
	return false
}

func (f *fakeStore) Workspaces(context.Context) ([]domain.Workspace, error) {
	out := make([]domain.Workspace, len(f.workspaces))
	copy(out, f.workspaces)
	return out, nil
}

func (f *fakeStore) Workspace(_ context.Context, id string) (domain.Workspace, error) {
	for _, w := range f.workspaces {
		if w.ID == id {
			return w, nil
		}
	}
	return domain.Workspace{}, domain.ErrNotFound
}

func (f *fakeStore) SaveWorkspace(_ context.Context, w domain.Workspace) error {
	for i := range f.workspaces {
		if f.workspaces[i].ID == w.ID {
			f.workspaces[i] = w
			return nil
		}
	}
	f.workspaces = append(f.workspaces, w)
	return nil
}

func (f *fakeStore) DeleteWorkspace(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	kept := f.workspaces[:0]
	for _, w := range f.workspaces {
		if w.ID != id {
			kept = append(kept, w)
		}
	}
	f.workspaces = kept
	return nil
}

// ActiveWorkspace answers the way the store does: the stored pointer when it names a row that is
// still there, and the default otherwise.
func (f *fakeStore) ActiveWorkspace(context.Context) (string, error) {
	if f.active != "" && f.has(f.active) {
		return f.active, nil
	}
	return domain.WorkspacePersonalID, nil
}

func (f *fakeStore) SetActiveWorkspace(_ context.Context, id string) error {
	f.active = id
	return nil
}

// fakeNotifier keeps what was published, so a test can say the windows were told and what about.
type fakeNotifier struct {
	topics   []string
	payloads []any
}

func (f *fakeNotifier) Publish(topic string, payload any) {
	f.topics = append(f.topics, topic)
	f.payloads = append(f.payloads, payload)
}

func newUseCase() (*UseCase, *fakeStore, *fakeNotifier) {
	store := newFakeStore()
	notifier := &fakeNotifier{}
	return NewUseCase(store, platform.NewIDGen(), notifier), store, notifier
}

// made is the workspace a test just created, which is the one after the default the store starts
// with.
func made(t *testing.T, state domain.WorkspaceState) domain.Workspace {
	t.Helper()
	if len(state.Workspaces) < 2 {
		t.Fatalf("workspaces = %+v, want the default and the new one", state.Workspaces)
	}
	return state.Workspaces[len(state.Workspaces)-1]
}

func TestCreateAppendsToList(t *testing.T) {
	u, store, _ := newUseCase()
	ctx := context.Background()

	state, err := u.Create(ctx, CreateInput{Name: "Каталог внутренних API", Color: "purple"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(state.Workspaces) != 2 {
		t.Fatalf("workspaces = %d, want the default and the new one", len(state.Workspaces))
	}

	// The default keeps the first row: the list is drawn in the order the spaces were made.
	if !state.Workspaces[0].Personal {
		t.Errorf("the first workspace = %+v, want the one the app is born with", state.Workspaces[0])
	}

	added := made(t, state)
	if added.Name != "Каталог внутренних API" || added.Color != "purple" {
		t.Errorf("the new workspace = %+v, want the name and the colour it was given", added)
	}
	// The form has no kind: a space made here is a personal one, and the row says so.
	if added.Kind != domain.WorkspacePersonal {
		t.Errorf("kind = %q, want a personal space", added.Kind)
	}
	if added.Personal {
		t.Errorf("the new workspace reports itself as the default: %+v", added)
	}
	if added.ID == "" || added.ID == domain.WorkspacePersonalID {
		t.Errorf("id = %q, want one of its own", added.ID)
	}
	// A new space is empty, so the window is left where it was rather than moved into it.
	if state.ActiveID != domain.WorkspacePersonalID {
		t.Errorf("activeId = %q, want the window left where it was", state.ActiveID)
	}
	if len(store.workspaces) != 2 {
		t.Errorf("the store holds %d workspace(s), want the two", len(store.workspaces))
	}
}

func TestPersonalWorkspaceIsNotDeletable(t *testing.T) {
	u, store, _ := newUseCase()
	ctx := context.Background()

	_, err := u.Delete(ctx, domain.WorkspacePersonalID)
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("Delete of the default = %v, want domain.ErrNotAllowed", err)
	}
	if code := domain.CodeOf(err); code != domain.CodePersonalWorkspace {
		t.Errorf("code = %q, want %q", code, domain.CodePersonalWorkspace)
	}
	// The refusal is the use case's, and it never reaches the store.
	if len(store.deleted) != 0 {
		t.Errorf("the store was asked to delete %v, want nothing touched", store.deleted)
	}

	state, err := u.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(state.Workspaces) != 1 || state.ActiveID != domain.WorkspacePersonalID {
		t.Errorf("after the refusal = %+v, want the list exactly as it was", state)
	}
}

func TestDeleteFallsBackToTheDefault(t *testing.T) {
	u, store, notifier := newUseCase()
	ctx := context.Background()

	created, err := u.Create(ctx, CreateInput{Name: "Команда"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	target := made(t, created).ID
	if _, err := u.Switch(ctx, target); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	state, err := u.Delete(ctx, target)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if state.ActiveID != domain.WorkspacePersonalID {
		t.Errorf("activeId = %q, want the pointer back on the default", state.ActiveID)
	}
	if len(state.Workspaces) != 1 {
		t.Errorf("workspaces = %+v, want the default alone", state.Workspaces)
	}
	if store.active != domain.WorkspacePersonalID {
		t.Errorf("the stored pointer = %q, want the default written down", store.active)
	}

	// The window is told, and about the space it landed in rather than the one that went away.
	if len(notifier.payloads) == 0 {
		t.Fatal("nothing was published")
	}
	last := notifier.payloads[len(notifier.payloads)-1]
	changed, ok := last.(Changed)
	if !ok {
		t.Fatalf("the last event carried %T, want Changed", last)
	}
	if changed.Workspace.ID != domain.WorkspacePersonalID {
		t.Errorf("the last event named %q, want the default", changed.Workspace.ID)
	}
	if topic := notifier.topics[len(notifier.topics)-1]; topic != TopicChanged {
		t.Errorf("the last topic = %q, want %q", topic, TopicChanged)
	}
}

func TestSwitchStoresAndPublishes(t *testing.T) {
	u, store, notifier := newUseCase()
	ctx := context.Background()

	created, err := u.Create(ctx, CreateInput{Name: "Команда"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	target := made(t, created).ID

	state, err := u.Switch(ctx, target)
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if store.active != target {
		t.Errorf("the stored pointer = %q, want %q", store.active, target)
	}
	if state.ActiveID != target {
		t.Errorf("activeId = %q, want %q", state.ActiveID, target)
	}

	if len(notifier.topics) != 1 || notifier.topics[0] != TopicChanged {
		t.Fatalf("topics = %v, want the one %q", notifier.topics, TopicChanged)
	}
	changed, ok := notifier.payloads[0].(Changed)
	if !ok {
		t.Fatalf("the event carried %T, want Changed", notifier.payloads[0])
	}
	if changed.Workspace.ID != target {
		t.Errorf("the event named %q, want %q", changed.Workspace.ID, target)
	}

	// Switching to a row that is not there changes nothing and says so.
	if _, err := u.Switch(ctx, "нет-такого"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("switching to a space that is not there = %v, want ErrNotFound", err)
	}
	if store.active != target {
		t.Errorf("the pointer moved to %q on a failed switch, want it left at %q", store.active, target)
	}
}

func TestUpdateRenamesAndRecolors(t *testing.T) {
	u, _, notifier := newUseCase()
	ctx := context.Background()

	// The default is the one on screen, so this is also the case where the windows have to hear it.
	name := "Личное"
	state, err := u.Update(ctx, domain.WorkspacePersonalID, Patch{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := state.Workspaces[0].Name; got != name {
		t.Errorf("name = %q, want %q", got, name)
	}
	// A field the patch leaves out is left as it is, which is what makes the patch partial.
	if got := state.Workspaces[0].Color; got != "blue" {
		t.Errorf("color = %q, want it left as it was", got)
	}

	color := "purple"
	state, err = u.Update(ctx, domain.WorkspacePersonalID, Patch{Color: &color})
	if err != nil {
		t.Fatalf("Update (colour): %v", err)
	}
	got := state.Workspaces[0]
	if got.Color != "purple" || got.Name != name {
		t.Errorf("after the second patch = %+v, want both changes kept", got)
	}
	if got.UpdatedAt == 0 {
		t.Error("updatedAt was not written")
	}

	if len(notifier.topics) != 2 || notifier.topics[0] != TopicChanged {
		t.Errorf("topics = %v, want one %q per change of the space on screen",
			notifier.topics, TopicChanged)
	}

	if _, err := u.Update(ctx, "нет-такого", Patch{Name: &name}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("updating a space that is not there = %v, want ErrNotFound", err)
	}
}

func TestNameValidation(t *testing.T) {
	u, store, _ := newUseCase()
	ctx := context.Background()

	for _, bad := range []string{"", "   ", strings.Repeat("x", maxNameLength+1)} {
		if _, err := u.Create(ctx, CreateInput{Name: bad}); !errors.Is(err, domain.ErrNotAllowed) {
			t.Errorf("Create(%q) = %v, want domain.ErrNotAllowed", bad, err)
		}
	}
	// The two refusals are told apart by their code, because the window words them differently.
	if _, err := u.Create(ctx, CreateInput{Name: ""}); domain.CodeOf(err) != domain.CodeNameEmpty {
		t.Errorf("an empty name = %q, want %q", domain.CodeOf(err), domain.CodeNameEmpty)
	}
	tooLong := strings.Repeat("x", maxNameLength+1)
	if _, err := u.Create(ctx,
		CreateInput{Name: tooLong}); domain.CodeOf(err) != domain.CodeNameTooLong {
		t.Errorf("a name past the limit = %q, want %q", domain.CodeOf(err), domain.CodeNameTooLong)
	}
	// Nothing was made by any of the refusals.
	if len(store.workspaces) != 1 {
		t.Errorf("the store holds %d workspace(s), want the default alone", len(store.workspaces))
	}

	// The limit counts characters and not bytes: forty Cyrillic letters are inside it.
	if _, err := u.Create(ctx, CreateInput{Name: strings.Repeat("я", maxNameLength)}); err != nil {
		t.Errorf("a name of exactly %d characters = %v, want it accepted", maxNameLength, err)
	}
	// A name is trimmed rather than refused for the space around it.
	state, err := u.Create(ctx, CreateInput{Name: "  Команда  "})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := made(t, state).Name; got != "Команда" {
		t.Errorf("name = %q, want it trimmed", got)
	}

	// A patch is held to the same rule as a create.
	empty := "   "
	if _, err := u.Update(ctx, domain.WorkspacePersonalID,
		Patch{Name: &empty}); domain.CodeOf(err) != domain.CodeNameEmpty {
		t.Errorf("renaming to an empty name = %q, want %q", domain.CodeOf(err), domain.CodeNameEmpty)
	}
}
