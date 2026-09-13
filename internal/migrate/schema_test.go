package migrate

import (
	"testing"

	"json-inspector/migrations"

	_ "modernc.org/sqlite"
)

// This file is the only place that runs the real embedded schema instead of a fixture. It is
// what proves the .sql files shipped inside the binary actually parse, run in order, and leave
// behind the schema the rest of the app queries by name.

// expectedTables is every table the app reads, including the runner's own ledger. A rename or a
// dropped file shows up here rather than as a "no such table" at runtime.
var expectedTables = []string{
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
	if len(result.Applied) != 8 {
		t.Errorf("applied %d migrations, want 8", len(result.Applied))
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
// per-connection pragma that SQLite leaves off by default, and with it off every REFERENCES
// clause in the schema is accepted but silently ignored — all the ON DELETE CASCADE wiring in
// records, collection_nodes and the script tables would quietly do nothing, and orphaned rows
// would only surface much later as missing data.
//
// So this asserts the path end to end: the pragma reports 1, a dangling child row is refused,
// and a delete really does cascade.
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
		`INSERT INTO records (id, source, method, url, started_at) VALUES (?, ?, ?, ?, ?)`,
		"rec-1", "manual", "GET", "https://example.test/", 1); err != nil {
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
