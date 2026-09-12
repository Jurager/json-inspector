-- Collections and the request tree inside them.
--
-- Folders and requests share one table so that moving a node is a parent_id/position update
-- rather than a delete-and-reinsert across two tables; the kind column carries the CHECK that
-- keeps the request-only columns (method, url, body) meaningful.
--
-- parent_id is the cascade path that deletes a folder's whole subtree in one statement.

CREATE TABLE collections (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  auth_json TEXT,
  scripts_json TEXT,
  position INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE collection_nodes (
  id TEXT PRIMARY KEY,
  collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  parent_id TEXT REFERENCES collection_nodes(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK (kind IN ('folder','request')),
  name TEXT NOT NULL,
  position INTEGER NOT NULL,
  description TEXT,
  auth_json TEXT,
  scripts_json TEXT,
  method TEXT,
  url TEXT,
  params_json TEXT,
  headers_json TEXT,
  body TEXT,
  body_kind TEXT,
  cookies_json TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE INDEX collection_nodes_tree ON collection_nodes(collection_id, parent_id, position);
