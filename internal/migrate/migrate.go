// Package migrate applies SQL migrations to a SQLite database.
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

// ErrChecksumMismatch reports that an applied migration was modified.
var ErrChecksumMismatch = errors.New("migration checksum mismatch")

// ErrDuplicateVersion reports that multiple migrations use the same version.
var ErrDuplicateVersion = errors.New("duplicate migration version")

const ledgerDDL = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version      INTEGER PRIMARY KEY,
    name         TEXT    NOT NULL,
    checksum     TEXT    NOT NULL,
    applied_at   INTEGER NOT NULL,
    execution_ms INTEGER NOT NULL
)`

// migrationName matches NNNN_name.sql migration files.
var migrationName = regexp.MustCompile(`^([0-9]{4})_(.+)\.sql$`)

// Migration is a SQL migration loaded from a fs.FS.
type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum string
}

// Result reports the migrations applied and skipped by Up.
type Result struct {
	Applied []Migration
	Skipped int
}

// Load reads and sorts migrations by version.
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
			return nil, fmt.Errorf(
				"%w: version %04d is claimed by both %s and %s",
				ErrDuplicateVersion, version, previous, entry.Name(),
			)
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

	sort.Slice(out, func(i, j int) bool {
		return out[i].Version < out[j].Version
	})

	return out, nil
}

// Up applies all pending migrations in version order.
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

	var result Result
	pending := make([]Migration, 0, len(migrations))

	for _, m := range migrations {
		checksum, applied := recorded[m.Version]
		if !applied {
			pending = append(pending, m)
			continue
		}

		if checksum != m.Checksum {
			return Result{}, fmt.Errorf(
				"applying migration %04d_%s: %w",
				m.Version, m.Name, ErrChecksumMismatch,
			)
		}

		result.Skipped++
	}

	for _, m := range pending {
		if err := apply(ctx, db, m); err != nil {
			return Result{}, fmt.Errorf(
				"applying migration %04d_%s: %w",
				m.Version, m.Name, err,
			)
		}
		result.Applied = append(result.Applied, m)
	}

	return result, nil
}

// appliedChecksums reads applied migration checksums.
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

// apply executes a migration and records it in the same transaction.
func apply(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	started := time.Now()

	if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
		return fmt.Errorf("executing: %w", err)
	}

	elapsed := time.Since(started).Milliseconds()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, checksum, applied_at, execution_ms) VALUES (?, ?,
			?, ?, ?)`,
		m.Version, m.Name, m.Checksum, time.Now().UnixMilli(), elapsed,
	)
	if err != nil {
		return fmt.Errorf("recording: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	return nil
}
