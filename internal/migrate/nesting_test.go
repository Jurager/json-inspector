package migrate

import (
	"database/sql"
	"io/fs"
	"strconv"
	"testing"
	"testing/fstest"

	"json-inspector/migrations"

	_ "modernc.org/sqlite"
)

// This file seeds the state the schema left behind while a folder was a row of collection_nodes and
// runs 0012 against it. The shape being checked is a destination, not a snapshot: a folder becomes a
// collection with the same id, its contents become that collection's rows, and the run history that
// named it as a folder still names it.

// upTo applies the embedded migrations up to and including version, so a test can reach the schema an
// older build left behind and seed real rows into it.
func upTo(t *testing.T, db *sql.DB, version int) {
	t.Helper()

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("listing embedded migrations: %v", err)
	}

	staged := fstest.MapFS{}
	for _, entry := range entries {
		match := migrationName.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		number, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatalf("parsing the version of %s: %v", entry.Name(), err)
		}
		if number > version {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, entry.Name())
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		staged[entry.Name()] = &fstest.MapFile{Data: body}
	}

	mustUp(t, db, staged)
}

// seedNestedFolders writes the tree the old schema allowed: a folder inside a folder inside a
// collection, a request at each of those levels, a script and an auth on each level, and a run that
// was started from a folder.
func seedNestedFolders(t *testing.T, db *sql.DB) {
	t.Helper()

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatalf("seeding %q: %v", query, err)
		}
	}

	exec(`INSERT INTO collections (id, name, description, position, auth_json, scripts_json,
	                              created_at, updated_at)
	      VALUES ('col-1', 'Коллекция', '', 0, NULL, NULL, 1, 1)`)

	node := `INSERT INTO collection_nodes (id, collection_id, parent_id, kind, name, position,
	                                       description, auth_json, scripts_json, method, url,
	                                       created_at, updated_at)
	         VALUES (?, 'col-1', ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1)`

	// The folders carry a description, an auth and a script of their own: these are exactly the parts
	// of a folder that are not a request, and the whole point of the move is that they arrive.
	exec(node, "f-1", nil, "folder", "Внешняя", 0, "описание", `{"type":"bearer","token":"outer"}`,
		`{"pre":"1"}`, nil, nil)
	exec(node, "f-2", "f-1", "folder", "Внутренняя", 0, "", `{"type":"bearer","token":"inner"}`,
		`{"post":"2"}`, nil, nil)
	exec(node, "r-1", nil, "request", "Сверху", 1, nil, nil, nil, "GET", "https://root.test/")
	exec(node, "r-2", "f-2", "request", "Внутри", 0, nil, nil, nil, "POST", "https://deep.test/")

	// One run started from a folder and one from a request: only the first one changes meaning.
	exec(`INSERT INTO collection_runs (id, collection_id, node_id, started_at, finished_at,
	                                  passed, failed, duration_us)
	      VALUES ('run-1', 'col-1', 'f-2', 10, 20, 1, 0, 500)`)
	exec(`INSERT INTO collection_runs (id, collection_id, node_id, started_at, finished_at,
	                                  passed, failed, duration_us)
	      VALUES ('run-2', 'col-1', 'r-1', 10, 20, 1, 0, 500)`)
}

// column reports whether a table has a column, by asking what a query naming it says.
func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()

	_, err := db.Exec(`SELECT ` + column + ` FROM ` + table + ` LIMIT 0`)
	return err == nil
}

// TestFoldersBecomeCollections is the migration's reason for existing: after it, a folder is a
// collection with a parent, its contents belong to it, and nothing a level carried was left behind.
func TestFoldersBecomeCollections(t *testing.T) {
	db := testDB(t)
	// The app runs with the pragma on (it comes from the DSN), and the table rebuild is the part that
	// could break under it, so the migration is exercised the way the app runs it.
	if _, err := db.Exec(`PRAGMA foreign_keys = 1`); err != nil {
		t.Fatalf("enabling foreign keys: %v", err)
	}

	upTo(t, db, 11)
	seedNestedFolders(t, db)

	if _, err := Up(t.Context(), db, migrations.FS); err != nil {
		t.Fatalf("Up(migrations.FS): %v", err)
	}

	// A folder is now a collection with its own row: the id it had, its name, and its parent named by
	// the collection it was in, or by the folder it was in.
	type collectionRow struct {
		parent  sql.NullString
		name    string
		desc    string
		auth    sql.NullString
		scripts sql.NullString
		pos     int64
	}
	read := func(id string) collectionRow {
		t.Helper()
		var row collectionRow
		err := db.QueryRow(
			`SELECT parent_id, name, description, auth_json, scripts_json, position
			   FROM collections WHERE id = ?`, id).
			Scan(&row.parent, &row.name, &row.desc, &row.auth, &row.scripts, &row.pos)
		if err != nil {
			t.Fatalf("reading collection %s: %v", id, err)
		}
		return row
	}

	outer := read("f-1")
	if !outer.parent.Valid || outer.parent.String != "col-1" {
		t.Errorf("Внешняя sits in %v, want col-1", outer.parent)
	}
	if outer.name != "Внешняя" || outer.desc != "описание" {
		t.Errorf("Внешняя = %q/%q, want its own name and description", outer.name, outer.desc)
	}
	if !outer.auth.Valid || outer.auth.String != `{"type":"bearer","token":"outer"}` {
		t.Errorf("Внешняя kept %v as its auth, want what the folder had", outer.auth)
	}
	if !outer.scripts.Valid || outer.scripts.String != `{"pre":"1"}` {
		t.Errorf("Внешняя kept %v as its scripts, want what the folder had", outer.scripts)
	}

	inner := read("f-2")
	if !inner.parent.Valid || inner.parent.String != "f-1" {
		t.Errorf("Внутренняя sits in %v, want f-1: a folder in a folder is a collection in a collection",
			inner.parent)
	}
	if !inner.scripts.Valid || inner.scripts.String != `{"post":"2"}` {
		t.Errorf("Внутренняя kept %v as its scripts, want what the folder had", inner.scripts)
	}

	// The collection that was already one has no parent, and its position is untouched.
	if root := read("col-1"); root.parent.Valid {
		t.Errorf("col-1 now sits in %v, want the top level", root.parent)
	}

	// A request belongs to the collection that holds it, and nothing else about it moved.
	var (
		collectionID string
		method       string
		position     int64
	)
	err := db.QueryRow(
		`SELECT collection_id, method, position FROM collection_nodes WHERE id = 'r-2'`).
		Scan(&collectionID, &method, &position)
	if err != nil {
		t.Fatalf("reading the request that was inside a folder: %v", err)
	}
	if collectionID != "f-2" {
		t.Errorf("the request inside Внутренняя belongs to %q, want f-2", collectionID)
	}
	if method != "POST" || position != 0 {
		t.Errorf("the request came out as %s at %d, want POST at its own position", method, position)
	}

	// A request at the top of a collection keeps belonging to it: step 4 must not have moved it.
	if err := db.QueryRow(
		`SELECT collection_id FROM collection_nodes WHERE id = 'r-1'`).Scan(&collectionID); err != nil {
		t.Fatalf("reading the request that was at the top: %v", err)
	}
	if collectionID != "col-1" {
		t.Errorf("the request at the top of the collection belongs to %q, want col-1", collectionID)
	}

	// The run history reads the way the new model asks for it: the folder's run is a run of that
	// collection as a whole, and a request's run is left alone.
	var runCollection, runNode string
	err = db.QueryRow(`SELECT collection_id, node_id FROM collection_runs WHERE id = 'run-1'`).
		Scan(&runCollection, &runNode)
	if err != nil {
		t.Fatalf("reading the run started from a folder: %v", err)
	}
	if runCollection != "f-2" || runNode != "" {
		t.Errorf("the folder's run reads as (%q, %q), want (f-2, empty)", runCollection, runNode)
	}
	err = db.QueryRow(`SELECT collection_id, node_id FROM collection_runs WHERE id = 'run-2'`).
		Scan(&runCollection, &runNode)
	if err != nil {
		t.Fatalf("reading the run started from a request: %v", err)
	}
	if runCollection != "col-1" || runNode != "r-1" {
		t.Errorf("the request's run reads as (%q, %q), want (col-1, r-1)", runCollection, runNode)
	}
}

// TestCollectionNodesStopsBeingATree pins the other half: the table holds requests, and the two
// columns that made it a tree are gone rather than left to rot.
func TestCollectionNodesStopsBeingATree(t *testing.T) {
	db := testDB(t)
	upTo(t, db, 11)
	seedNestedFolders(t, db)

	if _, err := Up(t.Context(), db, migrations.FS); err != nil {
		t.Fatalf("Up(migrations.FS): %v", err)
	}

	for _, gone := range []string{"kind", "parent_id"} {
		if columnExists(t, db, "collection_nodes", gone) {
			t.Errorf("collection_nodes still has %s", gone)
		}
	}

	var requests int
	if err := db.QueryRow(`SELECT count(*) FROM collection_nodes`).Scan(&requests); err != nil {
		t.Fatalf("counting nodes: %v", err)
	}
	if requests != 2 {
		t.Errorf("collection_nodes has %d rows, want the 2 requests: a folder is not a node any more",
			requests)
	}

	// Dropping the table took its index with it, and the rebuilt one has to have been put back.
	var indexes int
	err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master
		  WHERE type = 'index' AND name = 'collection_nodes_tree'`).Scan(&indexes)
	if err != nil {
		t.Fatalf("looking up the index: %v", err)
	}
	if indexes != 1 {
		t.Error("collection_nodes_tree is missing: the rebuild forgot to put the index back")
	}
}

// TestNestingCascadesThroughCollections checks that the new self-reference is wired the way the
// model needs: deleting a collection takes the collections inside it, and their requests, with it.
func TestNestingCascadesThroughCollections(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`PRAGMA foreign_keys = 1`); err != nil {
		t.Fatalf("enabling foreign keys: %v", err)
	}
	upTo(t, db, 11)
	seedNestedFolders(t, db)

	if _, err := Up(t.Context(), db, migrations.FS); err != nil {
		t.Fatalf("Up(migrations.FS): %v", err)
	}

	if _, err := db.Exec(`DELETE FROM collections WHERE id = 'col-1'`); err != nil {
		t.Fatalf("deleting the collection: %v", err)
	}

	var collections, nodes int
	if err := db.QueryRow(`SELECT count(*) FROM collections`).Scan(&collections); err != nil {
		t.Fatalf("counting collections: %v", err)
	}
	if collections != 0 {
		t.Errorf("%d collections survived: the nesting cascade is not firing", collections)
	}
	if err := db.QueryRow(`SELECT count(*) FROM collection_nodes`).Scan(&nodes); err != nil {
		t.Fatalf("counting nodes: %v", err)
	}
	if nodes != 0 {
		t.Errorf("%d requests survived their collection", nodes)
	}
}
