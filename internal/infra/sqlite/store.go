// Package sqlite is the app's database: one file, one connection, schema versioned by migrations.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"strings"

	// Registers the "sqlite" driver. Pure Go, which is what lets Windows keep CGO_ENABLED=0.
	_ "modernc.org/sqlite"

	"json-inspector/internal/migrate"
	"json-inspector/internal/platform"
)

// sidecarSuffixes are the files SQLite writes next to the database in WAL mode. They hold the
// same bytes, secrets included, so they get the same permissions.
var sidecarSuffixes = []string{"", "-wal", "-shm"}

type Store struct {
	db   *sql.DB
	path string
}

// NewStore records the connection settings. It opens nothing: sql.Open only parses the DSN, so a
// bad path surfaces as a startup failure the user can see instead of a panic while the graph is
// being built.
func NewStore(dataDir platform.DataDir) (*Store, error) {
	path := dataDir.DatabasePath()

	// Pragmas belong in the DSN, not in a db.Exec: foreign_keys applies per connection and a pool
	// hands out whichever one is free, so a pragma set once silently stops holding — which is how
	// every ON DELETE CASCADE below would quietly stop working. auto_vacuum only takes effect before
	// the first table exists: without it, pruning old records would free rows but not disk space.
	dsn := "file:" + path + "?" + strings.Join([]string{
		"_pragma=journal_mode(WAL)",
		"_pragma=foreign_keys(1)",
		"_pragma=busy_timeout(5000)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=auto_vacuum(INCREMENTAL)",
	}, "&")

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	// One connection: a single-user desktop app has no read concurrency worth a pool, and this
	// keeps SQLITE_BUSY out of the picture by construction.
	db.SetMaxOpenConns(1)
	return &Store{db: db, path: path}, nil
}

// Open makes the first real connection, which is what creates the file.
func (s *Store) Open(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("connecting to %s: %w", s.path, err)
	}
	s.restrictPermissions()
	return nil
}

func (s *Store) Migrate(ctx context.Context, fsys fs.FS) (migrate.Result, error) {
	return migrate.Up(ctx, s.db, fsys)
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Path() string {
	return s.path
}

// restrictPermissions keeps the database to its owner. Secrets are stored as plaintext, and the
// file is created by the driver with whatever the umask allows.
func (s *Store) restrictPermissions() {
	for _, suffix := range sidecarSuffixes {
		// Not every sidecar exists yet, and Windows ignores the mode — best effort is the whole
		// point, so nothing here is worth failing a launch over.
		_ = os.Chmod(s.path+suffix, 0o600)
	}
}
