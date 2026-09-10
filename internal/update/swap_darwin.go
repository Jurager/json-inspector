//go:build darwin

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// swapAndRelaunch installs a downloaded .app archive. The whole bundle is
// swapped — never the binary inside it — so the code signature stays intact.
// The new bundle is verified with codesign before it is moved into place.
func swapAndRelaunch(archivePath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	// exe is <appsDir>/json-inspector.app/Contents/MacOS/json-inspector
	macosDir := filepath.Dir(exe)
	bundleDir := filepath.Dir(filepath.Dir(macosDir))
	if !strings.HasSuffix(bundleDir, ".app") {
		return fmt.Errorf("not running from an .app bundle: %s", exe)
	}
	appsDir := filepath.Dir(bundleDir)

	// Stage next to the bundle so every rename stays on one filesystem.
	staging, err := os.MkdirTemp(appsDir, ".ji-update-*")
	if err != nil {
		return fmt.Errorf("cannot stage update: %w", err)
	}
	defer os.RemoveAll(staging)

	if out, err := exec.Command("ditto", "-x", "-k", archivePath, staging).CombinedOutput(); err != nil {
		return fmt.Errorf("extracting update: %s", strings.TrimSpace(string(out)))
	}

	newBundle := filepath.Join(staging, "json-inspector.app")
	if _, err := os.Stat(newBundle); err != nil {
		return fmt.Errorf("archive does not contain json-inspector.app")
	}

	if err := verifySignature(newBundle); err != nil {
		return err
	}

	oldBundle := bundleDir + ".old"
	_ = os.RemoveAll(oldBundle)
	if err := os.Rename(bundleDir, oldBundle); err != nil {
		return fmt.Errorf("replacing %s: %w", bundleDir, err)
	}
	if err := os.Rename(newBundle, bundleDir); err != nil {
		_ = os.Rename(oldBundle, bundleDir) // roll back
		return fmt.Errorf("installing %s: %w", bundleDir, err)
	}
	_ = os.RemoveAll(oldBundle)

	// Relaunch through LaunchServices so the app gets normal activation.
	if err := exec.Command("open", bundleDir).Start(); err != nil {
		return fmt.Errorf("relaunching: %w", err)
	}
	os.Exit(0)
	return nil
}

// verifySignature checks that the downloaded bundle carries an intact code
// signature. An unsigned or tampered bundle is rejected rather than swapped in.
func verifySignature(bundle string) error {
	out, err := exec.Command("codesign", "--verify", "--deep", "--strict", bundle).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("code signature verification failed: %s", msg)
	}
	return nil
}
