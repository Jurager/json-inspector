-- Environments and the variables that resolve against them.
--
-- Globals are not a table of their own: they are `variables` rows with scope_kind='globals'
-- and a NULL scope_id, so one query can express the "environment wins over globals" override
-- order and one editor can edit both. That NULL is why the unique and lookup indexes wrap
-- scope_id in ifnull() — SQLite treats NULLs as distinct inside a unique index, which would
-- otherwise let two globals share a name.
--
-- Both tables are scoped to a workspace, and the unique index leads with it: variables belong to
-- one space, so `baseUrl` is a name each space may take for itself, and a global is global to its
-- workspace rather than to the whole database. The cascade takes both away with the workspace.

CREATE TABLE environments (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  color TEXT,
  readonly INTEGER NOT NULL DEFAULT 0,
  position INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE variables (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  scope_kind TEXT NOT NULL CHECK (scope_kind IN ('environment','globals')),
  scope_id TEXT,
  name TEXT NOT NULL,
  value TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL CHECK (kind IN ('text','secret')),
  enabled INTEGER NOT NULL DEFAULT 1,
  position INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX variables_scope_name_uq
  ON variables(workspace_id, scope_kind, ifnull(scope_id,''), name);
CREATE INDEX variables_lookup ON variables(workspace_id, scope_kind, ifnull(scope_id,''), enabled);
CREATE INDEX environments_workspace ON environments(workspace_id, position);
