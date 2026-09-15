package migrate

import (
	"testing"

	// Registers the "sqlite" driver, which is what runs the embedded schema for real.
	_ "modernc.org/sqlite"

	"json-inspector/migrations"
)

// This file is the only place that runs the real embedded schema instead of a fixture. It is
// what proves the .sql files shipped inside the binary actually parse, run in order, and leave
// behind the schema the rest of the app queries by name.

// expectedTables is every table the app reads, including the runner's own ledger. A rename or a
// dropped file shows up here rather than as a "no such table" at runtime.
var expectedTables = []string{
	"workspaces",
	"environments", "variables",
	"records", "record_bodies",
	"collections", "collection_nodes",
	"script_runs", "script_logs", "test_results",
	"collection_runs", "collection_run_results",
	"settings", "drafts",
	"data_imports",
	"schema_migrations",
}

func TestEmbeddedSchemaApplies(t *testing.T) {
	db := testDB(t)

	result, err := Up(t.Context(), db, migrations.FS)
	if err != nil {
		t.Fatalf("Up(migrations.FS): %v", err)
	}
	if len(result.Applied) != 7 {
		t.Errorf("applied %d migrations, want 7", len(result.Applied))
	}
	if result.Skipped != 0 {
		t.Errorf("skipped %d migrations on a fresh database, want 0", result.Skipped)
	}

	for _, name := range expectedTables {
		if !tableExists(t, db, name) {
			t.Errorf("table %s is missing after migrating", name)
		}
	}

	// Relaunching the app must not re-run any of it.
	again, err := Up(t.Context(), db, migrations.FS)
	if err != nil {
		t.Fatalf("second Up(migrations.FS): %v", err)
	}
	if len(again.Applied) != 0 || again.Skipped != len(result.Applied) {
		t.Errorf("second run applied %d and skipped %d, want 0 and %d",
			len(again.Applied), again.Skipped, len(result.Applied))
	}
}

// TestEmbeddedSchemaEnforcesForeignKeys is the point of the schema test: foreign_keys is a
// per-connection pragma SQLite leaves off by default, and with it off every REFERENCES clause is
// accepted but silently ignored — the cascades would do nothing and orphaned rows would surface
// much later as missing data. So the path is asserted end to end: the pragma reports 1, a
// dangling child row is refused, a delete really cascades.
func TestEmbeddedSchemaEnforcesForeignKeys(t *testing.T) {
	db := testDB(t)

	if _, err := Up(t.Context(), db, migrations.FS); err != nil {
		t.Fatalf("Up(migrations.FS): %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = 1`); err != nil {
		t.Fatalf("enabling foreign keys: %v", err)
	}

	var enabled int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatalf("reading the foreign_keys pragma: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("PRAGMA foreign_keys = %d, want 1", enabled)
	}

	// No record with seq 999999 exists, so this child row has a dangling parent.
	_, err := db.Exec(`INSERT INTO record_bodies (record_seq, side) VALUES (?, ?)`,
		999999, "request")
	if err == nil {
		t.Fatal("a record_bodies row with no parent record was accepted: foreign keys are off")
	}

	// Rejecting inserts and cascading deletes are different code paths, so the cascade is
	// checked rather than inferred from the failure above.
	if _, err := db.Exec(
		`INSERT INTO records (id, workspace_id, source, method, url,
			started_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"rec-1", "personal", "manual", "GET", "https://example.test/", 1); err != nil {
		t.Fatalf("inserting a record: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO record_bodies (record_seq, side) VALUES (1, 'response')`); err != nil {
		t.Fatalf("inserting a body for it: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM records WHERE id = ?`, "rec-1"); err != nil {
		t.Fatalf("deleting the record: %v", err)
	}

	var bodies int
	if err := db.QueryRow(`SELECT count(*) FROM record_bodies`).Scan(&bodies); err != nil {
		t.Fatalf("counting bodies: %v", err)
	}
	if bodies != 0 {
		t.Errorf("%d bodies survived their record: ON DELETE CASCADE is not firing", bodies)
	}
}

// A workspace is the row everything else hangs off, and deleting one leans on every cascade in the
// schema at once. Each child reaches it through a different parent, so each is checked rather than
// assumed from the one that happens to be wired the same way.
func TestEmbeddedSchemaCascadesAWorkspace(t *testing.T) {
	db := testDB(t)

	if _, err := Up(t.Context(), db, migrations.FS); err != nil {
		t.Fatalf("Up(migrations.FS): %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = 1`); err != nil {
		t.Fatalf("enabling foreign keys: %v", err)
	}

	const ws = "team-1"
	if _, err := db.Exec(
		`INSERT INTO workspaces (id, kind, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		ws, "team", 1, 1, 1); err != nil {
		t.Fatalf("inserting a workspace: %v", err)
	}

	seed := []struct {
		what string
		sql  string
		args []any
	}{
		{"a record", `INSERT INTO records (id, workspace_id, source, method, url, started_at)
			VALUES ('rec-2', ?, 'browser', 'GET', 'https://example.test/', 2)`, []any{ws}},
		{"a record body", `INSERT INTO record_bodies (record_seq, side)
			SELECT seq, 'response' FROM records WHERE id = 'rec-2'`, nil},
		{"a script run", `INSERT INTO script_runs (id, workspace_id, scope, ok, created_at)
			VALUES ('run-2', ?, 'pre', 1, 2)`, []any{ws}},
		{"an environment", `INSERT INTO environments
			(id, workspace_id, name, position, created_at, updated_at)
			VALUES ('env-2', ?, 'Stage', 0, 2, 2)`, []any{ws}},
		{"a variable", `INSERT INTO variables
			(id, workspace_id, scope_kind, name, kind, position, created_at, updated_at)
			VALUES ('var-2', ?, 'globals', 'host', 'text', 0, 2, 2)`, []any{ws}},
		{"a collection", `INSERT INTO collections
			(id, workspace_id, name, position, created_at, updated_at)
			VALUES ('col-2', ?, 'Team', 0, 2, 2)`, []any{ws}},
		{"a node in it", `INSERT INTO collection_nodes
			(id, collection_id, name, position, created_at, updated_at)
			VALUES ('node-2', 'col-2', 'List', 0, 2, 2)`, nil},
		{"the draft", `INSERT INTO drafts (workspace_id, id, updated_at) VALUES (?, 'command-line', 2)`,
			[]any{ws}},
	}
	for _, s := range seed {
		if _, err := db.Exec(s.sql, s.args...); err != nil {
			t.Fatalf("inserting %s: %v", s.what, err)
		}
	}

	if _, err := db.Exec(`DELETE FROM workspaces WHERE id = ?`, ws); err != nil {
		t.Fatalf("deleting the workspace: %v", err)
	}

	for _, table := range []string{
		"records", "record_bodies", "script_runs", "environments",
		"variables", "collections", "collection_nodes", "drafts",
	} {
		var rows int
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&rows); err != nil {
			t.Fatalf("counting %s: %v", table, err)
		}
		if rows != 0 {
			t.Errorf("%d rows survived in %s: ON DELETE CASCADE is not firing", rows, table)
		}
	}

	// The workspace the app starts with is not collateral: it was there before, and it is there after.
	var kept int
	personal := `SELECT count(*) FROM workspaces WHERE id = 'personal'`
	if err := db.QueryRow(personal).Scan(&kept); err != nil {
		t.Fatalf("counting workspaces: %v", err)
	}
	if kept != 1 {
		t.Errorf("the default workspace is gone: %d rows", kept)
	}
}
