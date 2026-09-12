// Package migrate applies the project's .sql schema files to a SQLite database.
//
// It is hand-rolled rather than taken from a library because the app needs exactly one
// behaviour — "bring this file up to date, once, in order, at startup" — and the established
// runners each bring a CLI, a driver assumption, or a dependency tree this build (pure Go,
// CGO_ENABLED=0) would rather not carry.
//
// The ledger lives in the same database as the schema it describes, so a migration and its
// bookkeeping commit or roll back together and can never disagree.
package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// ErrChecksumMismatch reports that a migration file no longer hashes to what the ledger
// recorded for its version. The file was edited after it ran, so the database it is being
// applied to was built from different SQL than the one now on disk.
var ErrChecksumMismatch = errors.New("migration checksum mismatch")

// ErrDuplicateVersion reports that two files in one set claim the same version number. There
// is no safe order to pick between them, so the set is rejected instead of guessed at.
var ErrDuplicateVersion = errors.New("duplicate migration version")

// ledgerDDL creates the record of what has run. It is the only table this package owns.
const ledgerDDL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version      INTEGER PRIMARY KEY,
	name         TEXT    NOT NULL,
	checksum     TEXT    NOT NULL,
	applied_at   INTEGER NOT NULL,
	execution_ms INTEGER NOT NULL
)`

// Files are named NNNN_name.sql. The four-digit padding is required so that lexical and
// numeric ordering agree: an unpadded "10_x.sql" would sort before "2_y.sql".
var migrationName = regexp.MustCompile(`^([0-9]{4})_(.+)\.sql$`)

// Migration is one .sql file, read from an fs.FS and identified by its filename.
type Migration struct {
	Version int
	// Name is the part after the version, with the .sql suffix dropped: "init" for
	// "0001_init.sql". It is recorded alongside the version so the ledger stays readable.
	Name string
	// SQL is the file's contents, executed as one unit.
	SQL string
	// Checksum is the hex SHA-256 of SQL, used to detect a file edited after it was applied.
	Checksum string
}

// Result reports what an Up call did.
type Result struct {
	// Applied lists the migrations this call ran, in the order it ran them.
	Applied []Migration
	// Skipped counts the migrations already recorded in the ledger.
	Skipped int
}

// Load reads every NNNN_name.sql file from the root of fsys and returns them ordered by
// version, ascending.
//
// Entries that do not match the filename pattern are ignored, not rejected: the FS in
// production is an embed.FS that holds only migrations today, but pointing this at a directory
// that also carries a README or an editor's .bak file should not be fatal.
func Load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("listing migrations: %w", err)
	}

	out := make([]Migration, 0, len(entries))
	seen := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := migrationName.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}

		version, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, fmt.Errorf("parsing version of %s: %w", entry.Name(), err)
		}
		if previous, duplicate := seen[version]; duplicate {
			return nil, fmt.Errorf("%w: version %04d is claimed by both %s and %s",
				ErrDuplicateVersion, version, previous, entry.Name())
		}
		seen[version] = entry.Name()

		body, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		sum := sha256.Sum256(body)
		out = append(out, Migration{
			Version:  version,
			Name:     match[2],
			SQL:      string(body),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Up applies every migration in fsys that this database has not seen yet, and reports which
// ones ran. Calling it on an up-to-date database is a no-op, so callers may run it on every
// start.
//
// Each pending migration runs in its own transaction together with its ledger row. SQLite
// makes DDL transactional, so a file that fails part-way leaves the database at the previous
// version rather than half-migrated, and the versions that already applied in this run stay
// applied.
func Up(ctx context.Context, db *sql.DB, fsys fs.FS) (Result, error) {
	if _, err := db.ExecContext(ctx, ledgerDDL); err != nil {
		return Result{}, fmt.Errorf("creating schema_migrations: %w", err)
	}

	migrations, err := Load(fsys)
	if err != nil {
		return Result{}, err
	}

	recorded, err := appliedChecksums(ctx, db)
	if err != nil {
		return Result{}, err
	}

	// The ledger is verified in full before a single pending file runs. A mismatch means this
	// database was built from a different tree than the one on disk, and stacking new schema
	// changes on top of that disagreement only makes it harder to unpick.
	var result Result
	pending := make([]Migration, 0, len(migrations))
	for _, m := range migrations {
		checksum, applied := recorded[m.Version]
		if !applied {
			pending = append(pending, m)
			continue
		}
		if checksum != m.Checksum {
			return Result{}, fmt.Errorf("applying migration %04d_%s: %w",
				m.Version, m.Name, ErrChecksumMismatch)
		}
		result.Skipped++
	}

	for _, m := range pending {
		if err := apply(ctx, db, m); err != nil {
			return Result{}, fmt.Errorf("applying migration %04d_%s: %w", m.Version, m.Name, err)
		}
		result.Applied = append(result.Applied, m)
	}
	return result, nil
}

// appliedChecksums reads the ledger into a version -> checksum map.
func appliedChecksums(ctx context.Context, db *sql.DB) (map[int]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("reading schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]string)
	for rows.Next() {
		var (
			version  int
			checksum string
		)
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scanning schema_migrations: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading schema_migrations: %w", err)
	}
	return applied, nil
}

// apply runs one migration and writes its ledger row in the same transaction. Doing both under
// one commit is what keeps the ledger honest: a version can never be recorded for schema
// changes that rolled back.
func apply(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	// No-op once Commit has succeeded; the error is only interesting on the failure paths,
	// where it is reported through the returned error anyway.
	defer func() { _ = tx.Rollback() }()

	started := time.Now()
	if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
		return fmt.Errorf("executing: %w", err)
	}
	elapsed := time.Since(started).Milliseconds()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, checksum, applied_at, execution_ms)
		 VALUES (?, ?, ?, ?, ?)`,
		m.Version, m.Name, m.Checksum, time.Now().UnixMilli(), elapsed)
	if err != nil {
		return fmt.Errorf("recording: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing: %w", err)
	}
	return nil
}
