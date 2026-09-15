// Package platform provides shared application infrastructure.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir is the application's per-user data directory.
type DataDir string

// NewDataDir returns the application's data directory, creating it if needed.
func NewDataDir() (DataDir, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving the config directory: %w", err)
	}

	dir := filepath.Join(base, "json-inspector")
	// Restrict access because the database may contain sensitive data.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}

	return DataDir(dir), nil
}

// DatabasePath returns the path to the application's SQLite database.
func (d DataDir) DatabasePath() string {
	return filepath.Join(string(d), "app.db")
}
