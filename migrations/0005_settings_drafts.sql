-- App settings and the unsaved request draft.
--
-- settings is a plain key/value table because the set of knobs is small, changes between
-- releases, and is always read whole at startup — a column per setting would mean a migration
-- for every preference added.
--
-- drafts is deliberately a single row's worth of shape (id is a fixed key, revision bumps on
-- each save): the composer's state has to outlive a window reload, and revision lets a stale
-- frontend detect that it is about to overwrite newer state.

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
  cookies_json TEXT NOT NULL DEFAULT '[]',
  updated_at INTEGER NOT NULL
);
