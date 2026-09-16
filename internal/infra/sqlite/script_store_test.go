package sqlite

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
)

// A script belongs to a level — a collection, a node of one, or the draft the command line composes
// — and "not set here" is not the same as "nothing to run": the first is NULL and inherits, the
// second is an empty answer.
func TestScriptsRoundTrip(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTree(t, store)

	if scripts, err := store.Scripts(ctx, ws, "col-1"); err != nil || scripts != nil {
		t.Fatalf("a collection with no scripts = %+v, %v, want nothing", scripts, err)
	}

	written := &domain.Scripts{Pre: "pm.environment.set('started', Date.now());",
		Post: "pm.test('ok', () => pm.expect(pm.response.code).to.equal(200));"}
	if err := store.SaveScripts(ctx, ws, "col-1", written); err != nil {
		t.Fatalf("SaveScripts: %v", err)
	}
	if err := store.SaveScripts(ctx, ws, "r-1",
		&domain.Scripts{Post: "console.log('свой');"}); err != nil {
		t.Fatalf("SaveScripts: %v", err)
	}

	fromCollection, err := store.Scripts(ctx, ws, "col-1")
	if err != nil {
		t.Fatalf("Scripts: %v", err)
	}
	if fromCollection == nil || fromCollection.Pre != written.Pre ||
		fromCollection.Post != written.Post {
		t.Errorf("collection scripts = %+v, want what was written", fromCollection)
	}

	fromNode, err := store.Scripts(ctx, ws, "r-1")
	if err != nil {
		t.Fatalf("Scripts: %v", err)
	}
	if fromNode == nil || fromNode.Pre != "" || fromNode.Post != "console.log('свой');" {
		t.Errorf("node scripts = %+v, want its own", fromNode)
	}

	// A node read whole carries its scripts; a tree row does not, for the reason it carries no body.
	node, err := store.Node(ctx, "r-1")
	if err != nil {
		t.Fatalf("Node: %v", err)
	}
	if node.Scripts == nil || node.Scripts.Post != "console.log('свой');" {
		t.Errorf("node = %+v, want its scripts with it", node.Scripts)
	}
	tree, err := store.Collections(ctx, ws)
	if err != nil {
		t.Fatalf("Collections: %v", err)
	}
	if tree[0].Items[0].Scripts != nil {
		t.Error("a tree row came with scripts, which the list has no business carrying")
	}

	// Cleared is NULL again, and that is a different thing from an empty script.
	if err := store.SaveScripts(ctx, ws, "col-1", nil); err != nil {
		t.Fatalf("SaveScripts(nil): %v", err)
	}
	if scripts, err := store.Scripts(ctx, ws, "col-1"); err != nil || scripts != nil {
		t.Errorf("cleared scripts = %+v, %v, want nothing", scripts, err)
	}
	if err := store.SaveScripts(ctx, ws, "col-1", &domain.Scripts{}); err != nil {
		t.Fatalf("SaveScripts(empty): %v", err)
	}
	empty, err := store.Scripts(ctx, ws, "col-1")
	if err != nil {
		t.Fatalf("Scripts: %v", err)
	}
	if empty == nil || !empty.Empty() {
		t.Errorf("empty scripts = %+v, want an answer that says there is nothing to run", empty)
	}

	// The command line's request is a level too, and its code lives with the draft it is: nobody has
	// to save a collection for the code around a request to exist.
	if err := store.SaveDraft(ctx, ws,
		domain.Draft{ID: domain.DraftCommandLine, Method: "GET"}); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if scripts, err := store.Scripts(ctx, ws,
		string(domain.DraftCommandLine)); err != nil || scripts != nil {
		t.Errorf("a fresh draft's scripts = %+v, %v, want nothing", scripts, err)
	}
	if err := store.SaveScripts(ctx, ws, string(domain.DraftCommandLine),
		&domain.Scripts{Pre: "console.log('черновик');"}); err != nil {
		t.Fatalf("SaveScripts: %v", err)
	}
	fromDraft, err := store.Scripts(ctx, ws, string(domain.DraftCommandLine))
	if err != nil {
		t.Fatalf("Scripts: %v", err)
	}
	if fromDraft == nil || fromDraft.Pre != "console.log('черновик');" {
		t.Errorf("draft scripts = %+v, want what was written", fromDraft)
	}
	// Saving the draft itself — every keystroke in the command line does — leaves its code alone.
	if err := store.SaveDraft(ctx, ws,
		domain.Draft{ID: domain.DraftCommandLine, Method: "POST",
			URL: "https://api.example.com"}); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if scripts, err := store.Scripts(ctx, ws,
		string(domain.DraftCommandLine)); err != nil || scripts == nil {
		t.Errorf("draft scripts after a save = %+v, %v, want them where they were", scripts, err)
	}

	if err := store.SaveScripts(ctx, ws, "нет-такого", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("scripts of a level that does not exist = %v, want ErrNotFound", err)
	}
	if _, err := store.Scripts(ctx, ws, "нет-такого"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("reading scripts of a level that does not exist = %v, want ErrNotFound", err)
	}
}

func TestScriptRunRoundTrip(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	seedTree(t, store)

	// A run hangs off a record, and the window names records by id.
	if err := store.SaveRecord(ctx, ws, domain.Record{
		RecordSummary: domain.RecordSummary{ID: "rec-1", Source: domain.SourceManual, Method: "GET",
			URL: "https://api.example.com"},
	}); err != nil {
		t.Fatalf("SaveRecord: %v", err)
	}

	if runs, err := store.ScriptRuns(ctx, "rec-1"); err != nil || len(runs) != 0 {
		t.Fatalf("a record whose scripts never ran = %+v, %v, want nothing", runs, err)
	}

	for _, run := range []domain.ScriptRun{
		{
			ID: "run-1", RecordID: "rec-1", NodeID: "col-1", Scope: domain.ScriptPre,
			OK: true, DurationUs: 120, CreatedAt: 1,
			Logs: []domain.ScriptLog{{Level: "log", Message: "начали"}},
		},
		{
			ID: "run-2", RecordID: "rec-1", NodeID: "r-1", Scope: domain.ScriptPost,
			OK: false, Error: "pm.expect: 404 не 200", DurationUs: 340, CreatedAt: 2,
			Logs: []domain.ScriptLog{{Level: "error", Message: "упало"}},
			Tests: []domain.TestResult{
				{Name: "статус 200", Passed: false, Error: "получен 404", DurationUs: 5},
			},
		},
	} {
		if err := store.SaveScriptRun(ctx, ws, run); err != nil {
			t.Fatalf("SaveScriptRun: %v", err)
		}
	}

	runs, err := store.ScriptRuns(ctx, "rec-1")
	if err != nil {
		t.Fatalf("ScriptRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want both", len(runs))
	}
	if runs[0].Scope != domain.ScriptPre || !runs[0].OK || len(runs[0].Logs) != 1 {
		t.Errorf("first run = %+v, want the pre-request one with its line", runs[0])
	}
	if runs[1].Scope != domain.ScriptPost || runs[1].OK || runs[1].Error == "" {
		t.Errorf("second run = %+v, want the post one and why it failed", runs[1])
	}
	if len(runs[1].Tests) != 1 || runs[1].Tests[0].Passed || runs[1].Tests[0].Name != "статус 200" {
		t.Errorf("tests = %+v, want the failed assertion with its name", runs[1].Tests)
	}
	if runs[1].NodeID != "r-1" {
		t.Errorf("node = %q, want the request the script belongs to", runs[1].NodeID)
	}

	// Saving the same run again replaces what it said rather than adding to it.
	again := runs[0]
	again.Logs = []domain.ScriptLog{{Level: "log", Message: "одна строка"}}
	if err := store.SaveScriptRun(ctx, ws, again); err != nil {
		t.Fatalf("SaveScriptRun: %v", err)
	}
	runs, err = store.ScriptRuns(ctx, "rec-1")
	if err != nil {
		t.Fatalf("ScriptRuns: %v", err)
	}
	if len(runs[0].Logs) != 1 || runs[0].Logs[0].Message != "одна строка" {
		t.Errorf("logs = %+v, want the run as it was saved last", runs[0].Logs)
	}

	// A record that does not exist is not a record whose runs are missing: an empty list is an answer.
	if runs, err := store.ScriptRuns(ctx, "нет-такого"); err != nil || len(runs) != 0 {
		t.Errorf("runs of a record that does not exist = %+v, %v, want nothing", runs, err)
	}

	// The runs hang off the record: pruned history takes its reports with it, and the lines and
	// assertions go through the cascade as well — which is also what says foreign keys are really on.
	if err := store.DeleteRecords(ctx, []string{"rec-1"}); err != nil {
		t.Fatalf("DeleteRecords: %v", err)
	}
	for _, table := range []string{"script_runs", "script_logs", "test_results"} {
		var left int
		if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&left); err != nil {
			t.Fatalf("counting %s: %v", table, err)
		}
		if left != 0 {
			t.Errorf("%s kept %d rows of a deleted record", table, left)
		}
	}
}
