-- Script execution history: what ran, what it logged, what it asserted, and the roll-ups the
-- collection runner reports.
--
-- Logs and test results are child tables of a run rather than JSON blobs on it, so the UI can
-- page a long log without parsing it and so a failed assertion stays queryable. Both are
-- WITHOUT ROWID because they are only ever read by (run_id, position), never by rowid.
--
-- collection_run_results keeps node_id without a foreign key on purpose: a result must survive
-- the request node being deleted or moved after the run, since it describes what happened, not
-- what the collection looks like now.
--
-- Every duration is in microseconds, and the name carries the unit: a column called duration_ms
-- holding microseconds is a lie that survives review. See records for why milliseconds were the
-- wrong unit to begin with.
--
-- collection_runs.node_id names the level a run was started from — a collection or a request
-- inside it — and an empty one means the collection itself, which is what a run of the whole
-- collection is: a saved folder is a row of its own, not the absence of one. Without it the
-- overview could not tell "the last run of this folder" from "the last run of the collection".
--
-- collection_run_results.record_id names the record the row produced, so a click opens the answer
-- rather than an empty pane; a request a pre-request script stopped has no record at all and keeps
-- it empty. It is the same column script_runs carries, and for the same reason.

CREATE TABLE script_runs (
  id TEXT PRIMARY KEY,
  -- The workspace is carried here and not only inherited through the record: a run whose record
  -- never existed — a pre-request script that stopped the request, an attempt that failed before
  -- there was an answer — has no record_seq to hang off, and would otherwise outlive the deletion
  -- of the workspace it happened in.
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  record_seq INTEGER REFERENCES records(seq) ON DELETE CASCADE,
  collection_run_id TEXT,
  node_id TEXT,
  scope TEXT NOT NULL CHECK (scope IN ('pre','post')),
  ok INTEGER NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  duration_us INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);

CREATE INDEX script_runs_record ON script_runs(record_seq);
CREATE INDEX script_runs_workspace ON script_runs(workspace_id, created_at);

CREATE TABLE script_logs (
  run_id TEXT NOT NULL REFERENCES script_runs(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  level TEXT NOT NULL,
  message TEXT NOT NULL,
  PRIMARY KEY (run_id, position)
) WITHOUT ROWID;

CREATE TABLE test_results (
  run_id TEXT NOT NULL REFERENCES script_runs(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  name TEXT NOT NULL,
  passed INTEGER NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  duration_us INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (run_id, position)
) WITHOUT ROWID;

CREATE TABLE collection_runs (
  id TEXT PRIMARY KEY,
  collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  node_id TEXT NOT NULL DEFAULT '',
  started_at INTEGER NOT NULL,
  finished_at INTEGER NOT NULL DEFAULT 0,
  passed INTEGER NOT NULL DEFAULT 0,
  failed INTEGER NOT NULL DEFAULT 0,
  duration_us INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE collection_run_results (
  run_id TEXT NOT NULL REFERENCES collection_runs(id) ON DELETE CASCADE,
  node_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  status INTEGER,
  ok INTEGER NOT NULL,
  duration_us INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  record_id TEXT,
  PRIMARY KEY (run_id, position)
) WITHOUT ROWID;
