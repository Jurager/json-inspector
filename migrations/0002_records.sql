-- Captured requests and their bodies.
--
-- seq rather than id is the primary key: it is a monotonic capture order that stays meaningful
-- across imports, so the list can sort on it cheaply, while id remains the identifier the
-- frontend and the extension protocol already speak.
--
-- Every timing column is NOT NULL DEFAULT 0 with has_timing as the separate flag, because
-- "never measured" and "measured as zero" are different facts about a capture.

CREATE TABLE records (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL UNIQUE,
  source TEXT NOT NULL CHECK (source IN ('manual','browser')),
  method TEXT NOT NULL,
  url TEXT NOT NULL,
  status INTEGER NOT NULL DEFAULT 0,
  status_text TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  cancelled INTEGER NOT NULL DEFAULT 0,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  dns_ms INTEGER NOT NULL DEFAULT 0,
  connect_ms INTEGER NOT NULL DEFAULT 0,
  tls_ms INTEGER NOT NULL DEFAULT 0,
  wait_ms INTEGER NOT NULL DEFAULT 0,
  download_ms INTEGER NOT NULL DEFAULT 0,
  has_timing INTEGER NOT NULL DEFAULT 0,
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

CREATE INDEX records_recent ON records(started_at DESC, seq DESC);
CREATE INDEX records_source ON records(source, started_at DESC);
CREATE INDEX records_tab ON records(tab_id, started_at DESC);
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
