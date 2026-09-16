package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
)

// Install downloads the release for this platform, checks it against the published checksum and
// hands the archive to the platform's swap. On success it does not return.
func (s *Source) Install(ctx context.Context, version string) error {
	rel, err := s.release(ctx, "tags/"+url.PathEscape(version))
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

	// The checksum is read before the download rather than after: a release that publishes no
	// checksum is one this app has decided not to install, and there is no reason to pull several
	// megabytes to find that out.
	want, err := s.checksum(ctx, checksumURL, asset)
	if err != nil {
		return err
	}

	resp, err := s.get(ctx, assetURL, downloadTimeout)
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

	// The checksum comes over TLS from the same host as the archive, so this catches a truncated or
	// corrupted download rather than a forged release: whoever can serve a different archive can
	// serve a matching checksums.txt beside it. On macOS the bundle's own signature is what closes
	// that gap; on Windows and Linux it stays open, and the fix there is signing the release.
	if got := hex.EncodeToString(sum.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset, want, got)
	}

	return swapAndRelaunch(archivePath)
}
