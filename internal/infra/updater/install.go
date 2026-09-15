package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
)

// Installing a release: download it, check it against the release's own checksums file, and
// give the archive to the platform to swap in.

func Install(version string) error {
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
