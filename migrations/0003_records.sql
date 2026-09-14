-- Captured requests and their bodies.
--
-- seq rather than id is the primary key: it is a monotonic capture order that stays meaningful
-- across imports, so the list can sort on it cheaply, while id remains the identifier the
-- frontend and the extension protocol already speak.
--
-- A record belongs to one workspace, which is what makes "switch workspace" a swap of the list
-- rather than a filter someone has to remember to apply. The id stays unique across the whole
-- database: a body is read by the record's own id, and neither the window nor the protocol has a
-- workspace to hand to that lookup.
--
-- Every timing is in microseconds, because a millisecond was the wrong unit for a warm connection:
-- on a reused one DNS, connect and TLS do not happen at all, and even the body of a small response
-- arrives in less than a millisecond — in whole milliseconds each of those read as 0, and the
-- timings tab showed a live total above a row of zeros that looked like a failed measurement.
--
-- Nullable is what "this did not happen" means: nothing was dialled because the connection was
-- already there, or nothing came back at all. Zero means a phase was measured and took no
-- measurable time, which is a different fact about the capture. That is why duration_us is
-- NOT NULL — every request takes some time — while the phases beside it are.

CREATE TABLE records (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL UNIQUE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  source TEXT NOT NULL CHECK (source IN ('manual','browser')),
  method TEXT NOT NULL,
  url TEXT NOT NULL,
  status INTEGER NOT NULL DEFAULT 0,
  status_text TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  cancelled INTEGER NOT NULL DEFAULT 0,
  duration_us INTEGER NOT NULL DEFAULT 0,
  dns_us INTEGER,
  connect_us INTEGER,
  tls_us INTEGER,
  wait_us INTEGER,
  download_us INTEGER,
  request_bytes INTEGER NOT NULL DEFAULT 0,
  response_bytes INTEGER NOT NULL DEFAULT 0,
  request_headers_json TEXT NOT NULL DEFAULT '[]',
  response_headers_json TEXT NOT NULL DEFAULT '[]',
  request_cookies_json TEXT NOT NULL DEFAULT '[]',
  script_result_json TEXT NOT NULL DEFAULT '',
  started_at INTEGER NOT NULL,
  finished_at INTEGER NOT NULL DEFAULT 0,
  tab_id INTEGER,
  tab_title TEXT,
  tab_url TEXT,
  favicon_url TEXT
);

-- Each of them leads with the workspace: every list the app draws is one workspace's history, and
-- the count limit is spent inside it rather than across everything the database has ever seen.
CREATE INDEX records_recent ON records(workspace_id, started_at DESC, seq DESC);
CREATE INDEX records_source ON records(workspace_id, source, started_at DESC);
CREATE INDEX records_tab ON records(workspace_id, tab_id, started_at DESC);
CREATE INDEX records_url ON records(url);

-- Bodies sit in their own WITHOUT ROWID table keyed by (record_seq, side) so that listing
-- records never pulls body bytes through the page cache, and so a large body can be spilled to
-- disk (file_path) while its row keeps only the metadata. The composite key is exactly the
-- lookup the detail pane performs, which is what makes the split cheap.
CREATE TABLE record_bodies (
  record_seq INTEGER NOT NULL REFERENCES records(seq) ON DELETE CASCADE,
  side TEXT NOT NULL CHECK (side IN ('request','response')),
  content BLOB,
  file_path TEXT,
  encoding TEXT NOT NULL DEFAULT 'utf8',
  size INTEGER NOT NULL DEFAULT 0,
  truncated INTEGER NOT NULL DEFAULT 0,
  sha256 TEXT,
  PRIMARY KEY (record_seq, side)
) WITHOUT ROWID;
