package settings

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
)

type fakeStore map[string]string

func (f fakeStore) Settings(context.Context) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range f {
		out[k] = v
	}
	return out, nil
}

func (f fakeStore) SaveSetting(_ context.Context, key, value string) error {
	f[key] = value
	return nil
}

type published struct {
	topic   string
	payload any
}

type fakeNotifier struct {
	seen []published
}

func (f *fakeNotifier) Publish(topic string, payload any) {
	f.seen = append(f.seen, published{topic: topic, payload: payload})
}

func newUseCase() (*UseCase, fakeStore, *fakeNotifier) {
	store := fakeStore{}
	notifier := &fakeNotifier{}
	return NewUseCase(store, notifier), store, notifier
}

func TestSnapshotFallsBackToDefaults(t *testing.T) {
	uc, _, _ := newUseCase()

	got, err := uc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if got != domain.DefaultSettings() {
		t.Errorf("Snapshot = %+v, want the defaults", got)
	}
}

// A stored value from a build that no longer exists — or a hand-edited database — must not leave
// the window without a theme or a panel width.
func TestSnapshotIgnoresUnreadableValues(t *testing.T) {
	uc, store, _ := newUseCase()
	store[domain.SettingTheme] = "solarized"
	store[domain.SettingLanguage] = "de"
	store[domain.SettingInspectorWidth] = "wide"
	store[domain.SettingSideWidth] = "-40"
	store[domain.SettingHistoryRetention] = "forever-ish"
	store[domain.SettingInspectorOpen] = "maybe"

	got, err := uc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	want := domain.DefaultSettings()
	if got.Theme != want.Theme {
		t.Errorf("theme = %q, want the default", got.Theme)
	}
	if got.Language != want.Language {
		t.Errorf("language = %q, want the default", got.Language)
	}
	if got.InspectorWidth != want.InspectorWidth {
		t.Errorf("inspectorWidth = %d, want the default", got.InspectorWidth)
	}
	if got.SideWidth != domain.MinPanelWidth {
		t.Errorf("sideWidth = %d, want it clamped to the minimum", got.SideWidth)
	}
	if got.HistoryRetention != want.HistoryRetention {
		t.Errorf("retention = %q, want the default", got.HistoryRetention)
	}
	if got.InspectorOpen {
		t.Error("inspectorOpen = true, want the default (closed)")
	}
}

func TestSetThemeStoresAndPublishes(t *testing.T) {
	uc, store, notifier := newUseCase()

	got, err := uc.SetTheme(context.Background(), domain.ThemeDark)
	if err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	if got.Theme != domain.ThemeDark {
		t.Errorf("theme = %q, want dark back", got.Theme)
	}
	if store[domain.SettingTheme] != "dark" {
		t.Errorf("stored theme = %q, want dark", store[domain.SettingTheme])
	}

	// The event carries the choice: "system" is a question only a window can answer.
	if len(notifier.seen) != 1 {
		t.Fatalf("published %d events, want one", len(notifier.seen))
	}
	event := notifier.seen[0]
	if event.topic != TopicThemeChanged {
		t.Errorf("topic = %q, want %q", event.topic, TopicThemeChanged)
	}
	changed, ok := event.payload.(ThemeChanged)
	if !ok || changed.Theme != domain.ThemeDark {
		t.Errorf("payload = %#v, want a ThemeChanged carrying dark", event.payload)
	}

	if _, err := uc.SetTheme(context.Background(),
		"solarized"); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("unknown theme = %v, want ErrNotAllowed", err)
	}
	if len(notifier.seen) != 1 {
		t.Error("a refused theme was published anyway")
	}
}

func TestSetLanguageStoresAndPublishes(t *testing.T) {
	uc, store, notifier := newUseCase()

	got, err := uc.SetLanguage(context.Background(), domain.LanguageRU)
	if err != nil {
		t.Fatalf("SetLanguage: %v", err)
	}
	if got.Language != domain.LanguageRU {
		t.Errorf("language = %q, want ru back", got.Language)
	}
	if store[domain.SettingLanguage] != "ru" {
		t.Errorf("stored language = %q, want ru", store[domain.SettingLanguage])
	}

	// The event carries the choice: "system" is a question only the webview can answer.
	if len(notifier.seen) != 1 {
		t.Fatalf("published %d events, want one", len(notifier.seen))
	}
	event := notifier.seen[0]
	if event.topic != TopicLanguageChanged {
		t.Errorf("topic = %q, want %q", event.topic, TopicLanguageChanged)
	}
	changed, ok := event.payload.(LanguageChanged)
	if !ok || changed.Language != domain.LanguageRU {
		t.Errorf("payload = %#v, want a LanguageChanged carrying ru", event.payload)
	}

	// A language there is no catalogue for must not be stored: the window would fall back to English
	// and quietly disagree with what the settings screen shows.
	if _, err := uc.SetLanguage(context.Background(), "de"); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("unknown language = %v, want ErrNotAllowed", err)
	}
	if len(notifier.seen) != 1 {
		t.Error("a refused language was published anyway")
	}
}

func TestSetLayoutWritesOnlyWhatItIsGiven(t *testing.T) {
	uc, store, _ := newUseCase()
	ctx := context.Background()

	if _, err := uc.SetLayout(ctx, LayoutPatch{SideWidth: ptr(1000)}); err != nil {
		t.Fatalf("SetLayout: %v", err)
	}
	if store[domain.SettingSideWidth] != "720" {
		t.Errorf("sideWidth = %q, want it clamped to the maximum", store[domain.SettingSideWidth])
	}
	// Untouched settings stay absent rather than being written with a default.
	if _, ok := store[domain.SettingInspectorWidth]; ok {
		t.Error("SetLayout wrote a width it was not given")
	}

	open := true
	got, err := uc.SetLayout(ctx, LayoutPatch{InspectorOpen: &open, InspectorWidth: ptr(320)})
	if err != nil {
		t.Fatalf("SetLayout: %v", err)
	}
	if !got.InspectorOpen || got.InspectorWidth != 320 {
		t.Errorf("settings = %+v, want the inspector open at 320", got)
	}
	if got.SideWidth != domain.MaxPanelWidth {
		t.Errorf("sideWidth = %d, want the value saved before", got.SideWidth)
	}
}

// The side of the list is a choice, not a measurement: a value the window cannot lay out is refused
// rather than clamped, because "hidden-ish" would leave the panel somewhere nobody asked for.
func TestSetLayoutRefusesAnUnknownListSide(t *testing.T) {
	uc, store, _ := newUseCase()
	ctx := context.Background()

	side := domain.ListSideRight
	got, err := uc.SetLayout(ctx, LayoutPatch{ListSide: &side})
	if err != nil {
		t.Fatalf("SetLayout: %v", err)
	}
	if got.ListSide != domain.ListSideRight {
		t.Errorf("listSide = %q, want right back", got.ListSide)
	}
	if store[domain.SettingListSide] != "right" {
		t.Errorf("stored listSide = %q, want right", store[domain.SettingListSide])
	}

	bad := domain.ListSide("middle")
	_, err = uc.SetLayout(ctx, LayoutPatch{ListSide: &bad})
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("unknown side = %v, want ErrNotAllowed", err)
	}
	if code := domain.CodeOf(err); code != domain.CodeUnknownListSide {
		t.Errorf("code = %q, want %q", code, domain.CodeUnknownListSide)
	}
	if store[domain.SettingListSide] != "right" {
		t.Errorf("stored listSide = %q, want the refusal to leave it alone",
			store[domain.SettingListSide])
	}
}

func TestSetRetentionValidates(t *testing.T) {
	uc, store, _ := newUseCase()
	ctx := context.Background()

	if _, err := uc.SetRetention(ctx, domain.RetainWeek); err != nil {
		t.Fatalf("SetRetention: %v", err)
	}
	if store[domain.SettingHistoryRetention] != "7" {
		t.Errorf("stored retention = %q, want 7", store[domain.SettingHistoryRetention])
	}
	if _, err := uc.SetRetention(ctx, "whenever"); !errors.Is(err, domain.ErrNotAllowed) {
		t.Errorf("unknown retention = %v, want ErrNotAllowed", err)
	}
}

// Theme is what the window reads before it exists; it has to answer even on an empty database.
func TestThemeAnswersOnAFreshDatabase(t *testing.T) {
	uc, _, _ := newUseCase()

	got, err := uc.Theme(context.Background())
	if err != nil {
		t.Fatalf("Theme: %v", err)
	}
	if got != domain.ThemeSystem {
		t.Errorf("Theme = %q, want the default", got)
	}
}

// Language answers on the same terms, and its default is the one the settings screen offers: follow
// the system, not a language picked for the user.
func TestLanguageAnswersOnAFreshDatabase(t *testing.T) {
	uc, _, _ := newUseCase()

	got, err := uc.Language(context.Background())
	if err != nil {
		t.Fatalf("Language: %v", err)
	}
	if got != domain.LanguageSystem {
		t.Errorf("Language = %q, want the default", got)
	}
}

func ptr[T any](v T) *T { return &v }
