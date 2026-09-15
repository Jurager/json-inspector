// Package updater checks GitHub for a newer release, downloads it, verifies its checksum and
// hands the archive to the platform's own swap: the app replaces itself and starts again.
package updater

import "time"

const (
	repoOwner = "Jurager"
	repoName  = "json-inspector"

	checkInterval   = 24 * time.Hour
	checkTimeout    = 5 * time.Second
	downloadTimeout = 5 * time.Minute
)

// CurrentVersion is the running build's version, stamped into the binary by the release tag;
// CurrentBuild is the CI run number, empty for local builds.
var (
	CurrentVersion = "dev"
	CurrentBuild   = ""
)

// Info is the wire shape the frontend reads for the update events and the About window.
type Info struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	// When the check that produced this ran, in ms since the epoch; zero if none ever has.
	CheckedAt int64 `json:"checkedAt,omitempty"`
}

// Check fetches the latest release for a manual check. Unlike StartupCheck it is never
// skipped, but it does record the check: a manual check is what makes the next launch's
// throttled check unnecessary, and the About window shows when the last one ran.
func Check() (Info, error) {
	rel, err := fetchRelease("latest")
	if err != nil {
		return Info{}, err
	}
	now := time.Now()
	writeCheckState(checkState{LastCheck: now, Latest: rel.TagName})
	return Info{
		Available: isNewer(rel.TagName, CurrentVersion),
		Current:   CurrentVersion,
		Latest:    rel.TagName,
		CheckedAt: now.UnixMilli(),
	}, nil
}

// Status reports what the last check saw, without touching the network — that is what the
// About window shows when it opens. No record yet is not an error: it just has nothing to say.
func Status() (Info, error) {
	prev, err := readCheckState()
	if err != nil {
		return Info{}, nil
	}
	return Info{
		Available: isNewer(prev.Latest, CurrentVersion),
		Current:   CurrentVersion,
		Latest:    prev.Latest,
		CheckedAt: prev.LastCheck.UnixMilli(),
	}, nil
}

func StartupCheck() *Info {
	if CurrentVersion == "dev" {
		return nil
	}
	if prev, err := readCheckState(); err == nil && time.Since(prev.LastCheck) < checkInterval {
		return nil
	}

	rel, err := fetchRelease("latest")
	if err != nil {
		return nil
	}
	now := time.Now()
	writeCheckState(checkState{LastCheck: now, Latest: rel.TagName})

	if isNewer(rel.TagName, CurrentVersion) {
		return &Info{Available: true, Current: CurrentVersion, Latest: rel.TagName,
			CheckedAt: now.UnixMilli()}
	}
	return nil
}
