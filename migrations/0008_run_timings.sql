-- A run of a folder is a run of its own, and every timing the app writes is in microseconds.
--
-- Without the node a run was started from, the overview could not tell "the last run of this folder"
-- from "the last run of the whole collection", and the header of a folder would show the
-- collection's run. An empty node_id means the collection itself, which is what a run of the whole
-- collection is: a saved folder is a row of its own, not the absence of one.
--
-- The unit rename matches records: a duration carries its unit in its name, and a column called
-- duration_ms holding microseconds is a lie that survives review. Nothing has ever written these
-- four tables — the runner is being built now — so there is no value to convert.

ALTER TABLE collection_runs ADD COLUMN node_id TEXT NOT NULL DEFAULT '';

ALTER TABLE collection_runs RENAME COLUMN duration_ms TO duration_us;
ALTER TABLE collection_run_results RENAME COLUMN duration_ms TO duration_us;
ALTER TABLE script_runs RENAME COLUMN duration_ms TO duration_us;
ALTER TABLE test_results RENAME COLUMN duration_ms TO duration_us;
