package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

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

func fetchChecksum(checksumURL, asset string) (string, error) {
	if checksumURL == "" {
		return "", fmt.Errorf("no checksums.txt in release")
	}
	resp, err := httpGet(checksumURL, checkTimeout*2)
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
	req.Header.Set("User-Agent", "json-inspector/"+CurrentVersion)
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
