-- Bookkeeping for one-shot imports of external data (Postman collections, HAR files, ...).
--
-- source is the primary key because each importer may run at most once: a second run would
-- duplicate every node it created, and there is no natural key to deduplicate against. The row
-- is written as 'pending' before the import starts and flipped to 'done' or 'failed' after, so
-- a crash mid-import leaves a visible marker rather than a silently half-populated collection.

CREATE TABLE data_imports (
  source TEXT PRIMARY KEY,
  status TEXT NOT NULL CHECK (status IN ('pending','done','failed')),
  started_at INTEGER NOT NULL,
  finished_at INTEGER NOT NULL DEFAULT 0,
  detail TEXT NOT NULL DEFAULT ''
);
