// Package platform holds what every layer needs and none of them owns: where the app keeps its
// files, how ids are minted, how the build identifies itself. It depends on nothing but stdlib.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir is the app's own directory. A distinct type so it cannot be confused with any other
// string travelling through the dependency graph.
type DataDir string

// NewDataDir resolves the per-user config directory and creates it.
func NewDataDir() (DataDir, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving the config directory: %w", err)
	}
	dir := filepath.Join(base, "json-inspector")
	// 0700, not 0755: secrets live in the database as plaintext, so the directory is nobody
	// else's business.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	return DataDir(dir), nil
}

// DatabasePath is where the SQLite file lives, WAL sidecars included.
func (d DataDir) DatabasePath() string {
	return filepath.Join(string(d), "app.db")
}
