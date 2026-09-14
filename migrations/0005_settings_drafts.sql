-- App settings and the unsaved request draft.
--
-- settings is a plain key/value table because the set of knobs is small, changes between
-- releases, and is always read whole at startup — a column per setting would mean a migration
-- for every preference added.
--
-- drafts is deliberately a single row's worth of shape (id is a fixed key, revision bumps on
-- each save): the composer's state has to outlive a window reload, and revision lets a stale
-- frontend detect that it is about to overwrite newer state.
--
-- scripts_json on a draft is there because the command line can carry scripts of its own around
-- the send, and a request nobody has saved owns neither a collection nor a node to hang them on.
-- NULL means the same as it does on the tree's levels — "nothing set here", which for the command
-- line is "nothing to add"; an empty string would record "nothing" where "as it is" is meant.
--
-- body_kind, form_json and body_file are the draft's half of the five body formats the tree
-- carries; see collection_nodes for what they mean.

CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE drafts (
  id TEXT PRIMARY KEY,
  revision INTEGER NOT NULL DEFAULT 0,
  method TEXT NOT NULL DEFAULT 'GET',
  url TEXT NOT NULL DEFAULT '',
  params_json TEXT NOT NULL DEFAULT '[]',
  headers_json TEXT NOT NULL DEFAULT '[]',
  auth_json TEXT NOT NULL DEFAULT '{"type":"none"}',
  body TEXT NOT NULL DEFAULT '',
  body_kind TEXT NOT NULL DEFAULT 'raw',
  form_json TEXT NOT NULL DEFAULT '[]',
  body_file TEXT NOT NULL DEFAULT '',
  cookies_json TEXT NOT NULL DEFAULT '[]',
  scripts_json TEXT,
  updated_at INTEGER NOT NULL
);
