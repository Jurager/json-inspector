package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

const (
	checkTimeout    = 5 * time.Second
	downloadTimeout = 5 * time.Minute
)

// Source is the release source: this repository's releases on GitHub. It is the mechanism behind
// the update use case's Releases port and nothing else — when to look, which release is worth
// offering and what the user has already skipped are not its questions.
type Source struct {
	// userAgent carries the app's name and running version, which is what the API asks a client to
	// send. It comes from the build info rather than from a package variable, so there is one answer
	// to "what version is this" in the process.
	userAgent string
}

func NewSource(info platform.BuildInfo) *Source {
	return &Source{userAgent: platform.Slug + "/" + info.Version}
}

func releasesURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases",
		platform.RepoOwner, platform.Slug)
}

// assetName is the archive published for the running platform. The release is built per OS and
// architecture, so an update can only ever be the file that matches this one.
func assetName() string {
	switch runtime.GOOS {
	case "darwin":
		return fmt.Sprintf("%s-%s-%s.app.zip", platform.Slug, runtime.GOOS, runtime.GOARCH)
	case "windows":
		return fmt.Sprintf("%s-%s-%s.exe.zip", platform.Slug, runtime.GOOS, runtime.GOARCH)
	default:
		return fmt.Sprintf("%s-%s-%s.tar.gz", platform.Slug, runtime.GOOS, runtime.GOARCH)
	}
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	// Body is the release's own text. It is written by hand at release time and is the only place a
	// person can read what changed before installing it.
	Body   string        `json:"body"`
	Assets []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Latest asks for the newest release the channel allows.
//
// Stable asks GitHub for the release it calls latest, which by the API's own rule is the newest
// release that is not a pre-release. Beta lists them and takes the newest of any kind — GitHub
// orders that list newest first, so the first entry is the answer.
func (s *Source) Latest(ctx context.Context, channel domain.UpdateChannel) (domain.Release, error) {
	rel, err := s.newest(ctx, channel)
	if err != nil {
		return domain.Release{}, err
	}
	return domain.Release{Version: rel.TagName, Notes: releaseNotes(rel.Body)}, nil
}

func (s *Source) newest(ctx context.Context, channel domain.UpdateChannel) (*githubRelease, error) {
	if channel != domain.ChannelBeta {
		return s.release(ctx, "latest")
	}

	var listed []githubRelease
	if err := s.fetch(ctx, releasesURL(), checkTimeout, &listed); err != nil {
		return nil, err
	}
	if len(listed) == 0 {
		return nil, errors.New("the repository has no releases")
	}
	return &listed[0], nil
}

func (s *Source) release(ctx context.Context, subpath string) (*githubRelease, error) {
	var rel githubRelease
	if err := s.fetch(ctx, releasesURL()+"/"+subpath, checkTimeout, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// fetch reads one JSON document from the API. The limit is per response rather than shared: a
// release list is larger than a single release, and neither is something this app may read without
// a ceiling.
func (s *Source) fetch(
	ctx context.Context,
	endpoint string,
	timeout time.Duration,
	into any,
) error {
	resp, err := s.get(ctx, endpoint, timeout)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(into); err != nil {
		return fmt.Errorf("could not read the release: %w", err)
	}
	return nil
}

func (s *Source) checksum(ctx context.Context, checksumURL, asset string) (string, error) {
	if checksumURL == "" {
		return "", errors.New("no checksums.txt in release")
	}
	resp, err := s.get(ctx, checksumURL, checkTimeout*2)
	if err != nil {
		return "", fmt.Errorf("could not fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("could not read checksums: %w", err)
	}

	for _, line := range strings.Split(string(body), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum published for %s", asset)
}

func (s *Source) get(
	ctx context.Context,
	endpoint string,
	timeout time.Duration,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s returned %s", endpoint, resp.Status)
	}
	return resp, nil
}
