// Package update owns the policy of updating: when the app looks for a release, which one it
// offers, and what the user has already said about it. The network, the archive and the replacement
// of the running binary are the Releases port's — none of those can be exercised without a release
// to download, and this half has to be.
package update

import (
	"context"
	"errors"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// checkInterval is how long a successful check is trusted. A launch that finds a younger one skips
// its own, so opening the app twice does not ask the release source twice.
const checkInterval = 24 * time.Hour

// devVersion is what a build that was never released carries. Such a build has nothing to compare
// itself to — every published release is newer than it — so it never checks on its own, and says so
// rather than offering an update nobody can install.
const devVersion = "dev"

// Info is what a window is told about updating: the answer the last check produced, and where the
// running build stands. One shape for the event, for the status read and for the About window's own
// check, because the three are the same question asked from different places.
type Info struct {
	Available bool     `json:"available"`
	Current   string   `json:"current"`
	Latest    string   `json:"latest"`
	Notes     []string `json:"notes,omitempty"`
	CheckedAt int64    `json:"checkedAt,omitempty"`
}

type UseCase struct {
	releases Releases
	state    State
	settings Settings
	// version is the running build's, taken from the build info rather than from a package-level
	// variable: two copies of "what version am I" is one copy too many.
	version string
}

func NewUseCase(
	releases Releases,
	state State,
	settings Settings,
	info platform.BuildInfo,
) *UseCase {
	return &UseCase{releases: releases, state: state, settings: settings, version: info.Version}
}

// Check asks the release source for the newest release and answers whether it is worth offering,
// remembering what it found. A check that could not reach the source remembers nothing, so the next
// launch tries again instead of waiting out the interval on an answer nobody got.
func (u *UseCase) Check(ctx context.Context) (Info, error) {
	prefs, err := u.settings.Snapshot(ctx)
	if err != nil {
		return Info{}, err
	}

	release, err := u.releases.Latest(ctx, prefs.UpdateChannel)
	if err != nil {
		return Info{}, err
	}

	state, err := u.state.LoadUpdateState(ctx)
	if err != nil {
		return Info{}, err
	}
	state.LastCheckAt = time.Now().UnixMilli()
	state.Latest = release.Version
	state.Notes = release.Notes
	if err := u.state.SaveUpdateState(ctx, state); err != nil {
		return Info{}, err
	}

	return u.offer(state), nil
}

// Status reports what the last check saw, without touching the network: it is what the About window
// opens on, and what the status bar's link is rebuilt from after a restart. Never having checked is
// not an error — the window's line is simply empty until one runs.
func (u *UseCase) Status(ctx context.Context) (Info, error) {
	state, err := u.state.LoadUpdateState(ctx)
	if err != nil {
		return Info{}, err
	}
	if state.LastCheckAt == 0 {
		return Info{Current: u.version}, nil
	}
	return u.offer(state), nil
}

// StartupCheck is the launch-time check: silent about its own failure, and skipped outright
// whenever there is a reason to. It answers nil when there is nothing worth telling a window.
func (u *UseCase) StartupCheck(ctx context.Context) *Info {
	prefs, err := u.settings.Snapshot(ctx)
	if err != nil || !prefs.UpdateCheckAuto {
		return nil
	}
	if u.version == devVersion {
		return nil
	}

	state, err := u.state.LoadUpdateState(ctx)
	if err != nil {
		return nil
	}
	if state.LastCheckAt != 0 && time.Since(time.UnixMilli(state.LastCheckAt)) < checkInterval {
		return nil
	}

	info, err := u.Check(ctx)
	if err != nil || !info.Available {
		return nil
	}
	return &info
}

// Skip records that the user does not want to hear about this version again. It takes no argument
// on purpose: the version to skip is the one the last check found, and letting the window name it
// would be a way to skip something else. The answer is what the app now has to say, so the window
// that asked can draw the result of its own click without asking again.
func (u *UseCase) Skip(ctx context.Context) (Info, error) {
	state, err := u.state.LoadUpdateState(ctx)
	if err != nil {
		return Info{}, err
	}
	state.Skipped = state.Latest
	if err := u.state.SaveUpdateState(ctx, state); err != nil {
		return Info{}, err
	}
	return u.offer(state), nil
}

// Install downloads the release and hands it to the platform's swap. On success it does not return:
// the app replaces its own binary and starts again.
func (u *UseCase) Install(ctx context.Context, version string) error {
	if version == "" {
		return errors.New("no version to install")
	}
	return u.releases.Install(ctx, version)
}

// offer is the one place that decides whether what the last check found is worth showing. A skipped
// version is not offered — but only that version: the next release is news whether or not the one
// before it was skipped.
func (u *UseCase) offer(state domain.UpdateState) Info {
	info := Info{
		Current:   u.version,
		Latest:    state.Latest,
		CheckedAt: state.LastCheckAt,
	}
	if state.Latest == state.Skipped || !isNewer(state.Latest, u.version) {
		return info
	}
	info.Available = true
	info.Notes = state.Notes
	return info
}
