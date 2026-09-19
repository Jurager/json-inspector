-- A request can be pinned to an environment of its own, apart from the window's: the command
-- line's draft and a saved card each carry the pin beside everything else that makes the request.
-- Empty is "not pinned" — the same convention body_file already uses for an absent path — so a
-- request saved before this migration reads as following the window, which is what it always did.
ALTER TABLE drafts ADD COLUMN environment_id TEXT NOT NULL DEFAULT '';
ALTER TABLE collection_nodes ADD COLUMN environment_id TEXT;
