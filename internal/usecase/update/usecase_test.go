package update

import (
	"context"
	"errors"
	"testing"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// fakeReleases stands in for the network, the archive and the swap. It records what it was asked,
// which is how the tests below see the policy without a release to download.
type fakeReleases struct {
	release    domain.Release
	err        error
	asked      []domain.UpdateChannel
	installed  []string
	installErr error
}

func (f *fakeReleases) Latest(
	_ context.Context,
	channel domain.UpdateChannel,
) (domain.Release, error) {
	f.asked = append(f.asked, channel)
	if f.err != nil {
		return domain.Release{}, f.err
	}
	return f.release, nil
}

func (f *fakeReleases) Install(_ context.Context, version string) error {
	f.installed = append(f.installed, version)
	return f.installErr
}

type fakeState struct {
	state domain.UpdateState
	err   error
	// saves counts the writes, which is what tells "the check was remembered" apart from "the check
	// ran and the answer was thrown away".
	saves int
}

func (f *fakeState) LoadUpdateState(context.Context) (domain.UpdateState, error) {
	if f.err != nil {
		return domain.UpdateState{}, f.err
	}
	return f.state, nil
}

func (f *fakeState) SaveUpdateState(_ context.Context, state domain.UpdateState) error {
	if f.err != nil {
		return f.err
	}
	f.state = state
	f.saves++
	return nil
}

type fakeSettings struct {
	settings domain.Settings
	err      error
}

func (f *fakeSettings) Snapshot(context.Context) (domain.Settings, error) {
	if f.err != nil {
		return domain.Settings{}, f.err
	}
	return f.settings, nil
}

type harness struct {
	uc       *UseCase
	releases *fakeReleases
	state    *fakeState
	settings *fakeSettings
}

// newHarness builds the use case over fakes, with a running version the tests can compare against.
func newHarness(version string) *harness {
	releases := &fakeReleases{}
	state := &fakeState{}
	prefs := &fakeSettings{settings: domain.DefaultSettings()}
	return &harness{
		uc:       NewUseCase(releases, state, prefs, platform.BuildInfo{Version: version}),
		releases: releases,
		state:    state,
		settings: prefs,
	}
}

func TestCheckOffersANewerRelease(t *testing.T) {
	h := newHarness("1.0.0")
	h.releases.release = domain.Release{Version: "1.2.0", Notes: []string{"Fixed a thing"}}

	got, err := h.uc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !got.Available {
		t.Error("Available = false, want true for 1.2.0 over 1.0.0")
	}
	if got.Latest != "1.2.0" || got.Current != "1.0.0" {
		t.Errorf("Latest/Current = %q/%q, want 1.2.0/1.0.0", got.Latest, got.Current)
	}
	if len(got.Notes) != 1 {
		t.Errorf("Notes = %v, want the release's own line", got.Notes)
	}
	if h.state.saves != 1 || h.state.state.LastCheckAt == 0 {
		t.Error("the check was not remembered: the next launch would ask again immediately")
	}
}

func TestCheckDoesNotOfferTheSameOrAnOlderRelease(t *testing.T) {
	for _, version := range []string{"1.0.0", "0.9.0"} {
		h := newHarness("1.0.0")
		h.releases.release = domain.Release{Version: version}

		got, err := h.uc.Check(context.Background())
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if got.Available {
			t.Errorf("Available = true for %s over 1.0.0", version)
		}
		if len(got.Notes) != 0 {
			t.Error("notes were shown for a release that is not being offered")
		}
	}
}

// A check that could not reach the source must leave the clock alone: the timestamp is what holds
// the next launch back, and a failed check that stamped itself would silence the app for a day.
func TestAFailedCheckIsNotRemembered(t *testing.T) {
	h := newHarness("1.0.0")
	h.releases.err = errors.New("no network")

	if _, err := h.uc.Check(context.Background()); err == nil {
		t.Fatal("Check returned no error")
	}
	if h.state.saves != 0 || h.state.state.LastCheckAt != 0 {
		t.Errorf("the failed check was recorded: %+v", h.state.state)
	}
}

func TestStatusReportsWithoutAsking(t *testing.T) {
	h := newHarness("1.0.0")
	h.state.state = domain.UpdateState{
		LastCheckAt: time.Now().UnixMilli(),
		Latest:      "1.2.0",
		Notes:       []string{"Fixed a thing"},
	}

	got, err := h.uc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !got.Available || got.Latest != "1.2.0" {
		t.Errorf("Status = %+v, want the remembered release offered", got)
	}
	if len(h.releases.asked) != 0 {
		t.Error("Status reached the release source; it must answer from what is already known")
	}
}

// Never having checked is the state of every fresh install, and the window has to be able to open
// on it: the answer is an empty line, not a failure.
func TestStatusBeforeTheFirstCheckIsEmptyAndNotAnError(t *testing.T) {
	h := newHarness("1.0.0")

	got, err := h.uc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if got.Available || got.CheckedAt != 0 || got.Current != "1.0.0" {
		t.Errorf("Status = %+v, want only the running version", got)
	}
}

func TestStartupCheckSkipsWhenTheLastCheckIsYoung(t *testing.T) {
	h := newHarness("1.0.0")
	h.state.state.LastCheckAt = time.Now().Add(-time.Hour).UnixMilli()
	h.releases.release = domain.Release{Version: "1.2.0"}

	if got := h.uc.StartupCheck(context.Background()); got != nil {
		t.Errorf("StartupCheck = %+v, want nil inside the interval", got)
	}
	if len(h.releases.asked) != 0 {
		t.Error("the release source was asked despite a young check")
	}
}

func TestStartupCheckAsksAfterTheInterval(t *testing.T) {
	h := newHarness("1.0.0")
	h.state.state.LastCheckAt = time.Now().Add(-checkInterval - time.Hour).UnixMilli()
	h.releases.release = domain.Release{Version: "1.2.0"}

	got := h.uc.StartupCheck(context.Background())
	if got == nil {
		t.Fatal("StartupCheck = nil, want the release")
	}
	if !got.Available {
		t.Error("Available = false, want true")
	}
}

// Turning the automatic check off silences the launch-time one and nothing else: a check the user
// asks for is theirs to ask, and refusing it would be hiding the feature rather than switching it
// off.
func TestAutoCheckOffStopsOnlyTheLaunchTimeCheck(t *testing.T) {
	h := newHarness("1.0.0")
	h.settings.settings.UpdateCheckAuto = false
	h.releases.release = domain.Release{Version: "1.2.0"}

	if got := h.uc.StartupCheck(context.Background()); got != nil {
		t.Errorf("StartupCheck = %+v, want nil with the preference off", got)
	}
	if len(h.releases.asked) != 0 {
		t.Error("the release source was asked with the preference off")
	}

	if _, err := h.uc.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(h.releases.asked) != 1 {
		t.Errorf("asked %d times, want 1 from the manual check", len(h.releases.asked))
	}
}

// A build that was never released has nothing to compare itself to: every published version is
// newer, so offering one would tell every developer their own build is out of date.
func TestADevBuildNeverChecksOnItsOwn(t *testing.T) {
	h := newHarness(devVersion)
	h.releases.release = domain.Release{Version: "1.2.0"}

	if got := h.uc.StartupCheck(context.Background()); got != nil {
		t.Errorf("StartupCheck = %+v for a dev build, want nil", got)
	}
	if len(h.releases.asked) != 0 {
		t.Error("a dev build asked the release source")
	}
}

func TestCheckAsksTheChosenChannel(t *testing.T) {
	h := newHarness("1.0.0")
	h.settings.settings.UpdateChannel = domain.ChannelBeta
	h.releases.release = domain.Release{Version: "2.0.0-beta.1"}

	if _, err := h.uc.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(h.releases.asked) != 1 || h.releases.asked[0] != domain.ChannelBeta {
		t.Errorf("asked %v, want the beta channel", h.releases.asked)
	}
}

// Skipping answers for the version on screen and no other: the point of the button is to stop being
// told about one release, not to stop being told about releases.
func TestSkippingHidesThatVersionAndNoOther(t *testing.T) {
	h := newHarness("1.0.0")
	h.releases.release = domain.Release{Version: "1.2.0", Notes: []string{"Fixed a thing"}}
	if _, err := h.uc.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}

	got, err := h.uc.Skip(context.Background())
	if err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if got.Available {
		t.Error("the skipped version is still offered")
	}
	if h.state.state.Skipped != "1.2.0" {
		t.Errorf("Skipped = %q, want 1.2.0", h.state.state.Skipped)
	}

	// The next release is news again, even though it sits above the skipped one.
	h.releases.release = domain.Release{Version: "1.3.0", Notes: []string{"Added a thing"}}
	next, err := h.uc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !next.Available || next.Latest != "1.3.0" {
		t.Errorf("Check after skipping = %+v, want 1.3.0 offered", next)
	}
}

func TestSkipTakesTheVersionTheLastCheckFound(t *testing.T) {
	h := newHarness("1.0.0")
	h.state.state = domain.UpdateState{LastCheckAt: 1, Latest: "1.2.0"}

	got, err := h.uc.Skip(context.Background())
	if err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if got.Available {
		t.Error("Available = true right after skipping")
	}
	if got.Latest != "1.2.0" {
		t.Errorf("Latest = %q, want the version that was found", got.Latest)
	}
}

func TestInstallHandsTheVersionToTheMechanism(t *testing.T) {
	h := newHarness("1.0.0")

	if err := h.uc.Install(context.Background(), "1.2.0"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(h.releases.installed) != 1 || h.releases.installed[0] != "1.2.0" {
		t.Errorf("installed %v, want [1.2.0]", h.releases.installed)
	}
}

// The window only offers the button when a version is known, so an empty one is a caller that has
// gone wrong — and it must not reach the mechanism, where it would fail looking for an asset
// nobody asked for.
func TestInstallRefusesAnEmptyVersion(t *testing.T) {
	h := newHarness("1.0.0")

	if err := h.uc.Install(context.Background(), ""); err == nil {
		t.Fatal("Install with no version returned no error")
	}
	if len(h.releases.installed) != 0 {
		t.Error("an empty version reached the mechanism")
	}
}

// A store that cannot be read is a store that cannot answer, and every entry point says so rather
// than reporting an update nobody found.
func TestAStoreFailureIsReported(t *testing.T) {
	h := newHarness("1.0.0")
	h.state.err = errors.New("database is gone")

	if _, err := h.uc.Check(context.Background()); err == nil {
		t.Error("Check returned no error with an unreadable store")
	}
	if _, err := h.uc.Status(context.Background()); err == nil {
		t.Error("Status returned no error with an unreadable store")
	}
	if got := h.uc.StartupCheck(context.Background()); got != nil {
		t.Error("StartupCheck reported an update it could not have read")
	}
}
