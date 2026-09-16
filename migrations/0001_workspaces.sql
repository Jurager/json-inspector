-- Workspaces: the top-level container every other table now hangs off.
--
-- One database, one column per row — not a database per workspace. A workspace is what the whole
-- app is scoped to (history, collections, environments, the composer's draft), and the scope is a
-- plain foreign key, which is what a server would keep too: the same workspace_id travels to the
-- cloud, and "switch workspace" is a local pointer, not a second data set.
--
-- kind is what will tell a personal space from a team one; the team's members, invitations and
-- roles are a later table (workspace_members) and no column here has to change for them. color and
-- position are the switcher's own: the avatar tint and the order rows are listed in.
--
-- name is empty for the default workspace and that is deliberate: "Личное"/"Personal" is a word of
-- the interface, so it is chosen by the language the window is in, not frozen into the row by
-- whoever created it first. An empty name is drawn from the catalogue; a named one is drawn as it
-- was typed.
--
-- color is empty too, and empty means "no colour chosen" rather than "grey": a workspace nobody has
-- dressed looks like the app always has, and the window's glass takes a tint only once somebody
-- picks one.
--
-- active_environment_id has no foreign key on purpose: environments point at workspaces and a
-- reference back would be a cycle, and the column is a pointer into what the workspace holds, not a
-- second owner of it. The use case keeps it honest, the way `variables.scope_id` is kept honest.
--
-- The row below is the workspace the app has always had: it exists from the first launch, it is the
-- one the schema's own guardrails fall back to, and unlike the others it cannot be deleted.

CREATE TABLE workspaces (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL CHECK (kind IN ('personal','team')),
  color TEXT NOT NULL DEFAULT '',
  active_environment_id TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

INSERT INTO workspaces (id, name, kind, color, active_environment_id, position, created_at, updated_at)
VALUES ('personal', '', 'personal', '', '', 0, 0, 0);
