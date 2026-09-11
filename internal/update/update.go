package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner = "Jurager"
	repoName  = "json-inspector"

	checkInterval   = 24 * time.Hour
	checkTimeout    = 5 * time.Second
	downloadTimeout = 5 * time.Minute
)

var Current = "dev"

type Update struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
}

func releasesURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", repoOwner, repoName)
}

func assetName() string {
	switch runtime.GOOS {
	case "darwin":
		return fmt.Sprintf("json-inspector-%s-%s.app.zip", runtime.GOOS, runtime.GOARCH)
	case "windows":
		return fmt.Sprintf("json-inspector-%s-%s.exe.zip", runtime.GOOS, runtime.GOARCH)
	default:
		return fmt.Sprintf("json-inspector-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	}
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func fetchRelease(subpath string) (*githubRelease, error) {
	endpoint := releasesURL() + "/" + subpath
	resp, err := httpGet(endpoint, checkTimeout)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("could not read the release: %w", err)
	}
	return &rel, nil
}

func Check() (Update, error) {
	rel, err := fetchRelease("latest")
	if err != nil {
		return Update{}, err
	}
	return Update{
		Available: isNewer(rel.TagName, Current),
		Current:   Current,
		Latest:    rel.TagName,
	}, nil
}

func StartupCheck() *Update {
	if Current == "dev" {
		return nil
	}
	if state, err := readState(); err == nil && time.Since(state.LastCheck) < checkInterval {
		return nil
	}

	rel, err := fetchRelease("latest")
	if err != nil {
		return nil
	}
	writeState(state{LastCheck: time.Now(), Latest: rel.TagName})

	if isNewer(rel.TagName, Current) {
		return &Update{Available: true, Current: Current, Latest: rel.TagName}
	}
	return nil
}

func Apply(version string) error {
	rel, err := fetchRelease("tags/" + url.PathEscape(version))
	if err != nil {
		return err
	}

	asset := assetName()
	assetURL := ""
	checksumURL := ""
	for _, a := range rel.Assets {
		switch a.Name {
		case asset:
			assetURL = a.BrowserDownloadURL
		case "checksums.txt":
			checksumURL = a.BrowserDownloadURL
		}
	}
	if assetURL == "" {
		return fmt.Errorf("release %s has no asset for %s/%s", version, runtime.GOOS, runtime.GOARCH)
	}

	want, err := fetchChecksum(checksumURL, asset)
	if err != nil {
		return err
	}

	resp, err := httpGet(assetURL, downloadTimeout)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", asset, err)
	}
	defer resp.Body.Close()

	dir, err := os.MkdirTemp("", "json-inspector-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	archivePath := filepath.Join(dir, asset)
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	sum := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, sum), resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("downloading %s: %w", asset, err)
	}
	if err := f.Close(); err != nil {
		return err
	}

	if got := hex.EncodeToString(sum.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset, want, got)
	}

	return swapAndRelaunch(archivePath)
}

func fetchChecksum(url, asset string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("no checksums.txt in release")
	}
	resp, err := httpGet(url, checkTimeout*2)
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

func httpGet(endpoint string, timeout time.Duration) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "json-inspector/"+Current)
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

type state struct {
	LastCheck time.Time `json:"last_check"`
	Latest    string    `json:"latest"`
}

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".json-inspector.update.json"), nil
}

func readState() (state, error) {
	var s state
	path, err := statePath()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return state{}, err
	}
	return s, nil
}

func writeState(s state) {
	path, err := statePath()
	if err != nil {
		return
	}
	data, _ := json.Marshal(s)
	_ = os.WriteFile(path, data, 0o600)
}

func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		nums = append(nums, n)
	}
	return nums, true
}

func isNewer(candidate, current string) bool {
	c, okC := parseVersion(candidate)
	cur, okCur := parseVersion(current)
	if !okC || !okCur {
		return false
	}
	for i := 0; i < len(c) || i < len(cur); i++ {
		x, y := 0, 0
		if i < len(c) {
			x = c[i]
		}
		if i < len(cur) {
			y = cur[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}
