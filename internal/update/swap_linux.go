//go:build !darwin && !windows

package update

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// swapAndRelaunch installs a downloaded .tar.gz archive containing the single
// binary, atomically replacing the running executable and exec-ing into it.
func swapAndRelaunch(archivePath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	dir, err := os.MkdirTemp(filepath.Dir(exe), ".ji-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	if err := extractTarGz(archivePath, dir); err != nil {
		return fmt.Errorf("extracting update: %w", err)
	}
	newExe := filepath.Join(dir, "json-inspector")
	if _, err := os.Stat(newExe); err != nil {
		return fmt.Errorf("archive does not contain json-inspector")
	}
	if err := os.Chmod(newExe, 0o755); err != nil {
		return err
	}

	// Atomic swap over the running binary (the kernel keeps the old inode).
	if err := os.Rename(newExe, exe); err != nil {
		return fmt.Errorf("could not replace %s: %w", exe, err)
	}

	return syscall.Exec(exe, os.Args, os.Environ())
}
