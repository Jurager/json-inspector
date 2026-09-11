//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

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

	if err := extractZip(archivePath, dir); err != nil {
		return fmt.Errorf("extracting update: %w", err)
	}
	newExe := filepath.Join(dir, "json-inspector.exe")
	if _, err := os.Stat(newExe); err != nil {
		return fmt.Errorf("archive does not contain json-inspector.exe")
	}

	script := fmt.Sprintf(
		`ping -n 2 127.0.0.1 >nul & move /y "%s" "%s" & start "" "%s"`,
		newExe, exe, exe,
	)
	cmd := exec.Command("cmd", "/c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("relaunching: %w", err)
	}

	os.Exit(0)
	return nil
}
