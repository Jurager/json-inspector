// Package platform provides shared application infrastructure.
package platform

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DataDir is the application's per-user data directory.
type DataDir string

// NewDataDir is where the application's data directory goes. It resolves the path and creates
// nothing: creating the folder can fail — a locked path, a location a policy redirected — and that
// failure belongs on the startup screen beside the database's, not in a constructor that takes the
// app down before there is a window to show it in. Create does that, and the startup step is the
// caller: it is the one that can report the failure and be asked to try again.
func NewDataDir() (DataDir, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving the config directory: %w", err)
	}
	return DataDir(filepath.Join(base, DataDirName)), nil
}

// Create makes the directory. Access is restricted because the database holds secrets in plain
// text, and the file the app writes inside this folder is the one that has to stay readable only
// by its owner.
func (d DataDir) Create() error {
	if err := os.MkdirAll(string(d), 0o700); err != nil {
		// The reason on its own, not MkdirAll's whole error: it repeats the path and the verb, and
		// "creating X: mkdir X: Access is denied." says the same thing twice in the one line the
		// screen shows a person who is already looking at a problem.
		var failed *fs.PathError
		if errors.As(err, &failed) {
			err = failed.Err
		}
		return fmt.Errorf("creating %s: %w", d, err)
	}
	return nil
}

// DatabasePath returns the path to the application's SQLite database.
func (d DataDir) DatabasePath() string {
	return filepath.Join(string(d), "app.db")
}
