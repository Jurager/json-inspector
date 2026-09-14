-- Collections and the requests inside them.
--
-- A folder is a collection with a parent, and there is no second entity for it: the only thing
-- that used to distinguish the two was that a collection had no parent, so the tree understood
-- exactly one level of them. parent_id makes that level a property of the row instead of the
-- shape of the schema, and a node inside a collection is a request — folders are collections, so
-- nothing else needs a place in the node table.
--
-- A request keeps no parent of its own: it lives directly in the collection it belongs to (or in
-- a folder, which is one), so its place is collection_id plus position, moving it is one update,
-- and the cascade takes a folder's whole subtree with it.
--
-- auth_json and scripts_json are NULL on both tables, and NULL means "not set here": a request
-- with no auth of its own inherits from the nearest level above that answers, which is the same
-- reading the variable scopes use. An empty string would be a level that answered "nothing".
--
-- body_kind names one of five formats — json, xml, raw, form-data, binary; form_json is the grid
-- form-data is written in, and body_file the one path a binary body is read from. Columns rather
-- than a single blob because every other structured field here is a column too (params_json,
-- headers_json, cookies_json, auth_json, scripts_json), and one blob would be the only place
-- where the DDL hides its own contents.

CREATE TABLE collections (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  parent_id TEXT REFERENCES collections(id) ON DELETE CASCADE,
  auth_json TEXT,
  scripts_json TEXT,
  position INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE collection_nodes (
  id TEXT PRIMARY KEY,
  collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
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
  form_json TEXT,
  body_file TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE INDEX collection_nodes_tree ON collection_nodes(collection_id, position);
