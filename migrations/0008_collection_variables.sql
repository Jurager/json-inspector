-- Variables a collection answers for its own requests.
--
-- The environment is the set of variables a workspace works in; a collection's are the ones that
-- belong to the collection itself — a base URL, an API version — so that the tree can be handed to
-- somebody else and still mean the same thing under a different environment. They stand over the
-- environment's: a level that names a variable means that value for everything inside it, and the
-- same kind of variable it already is, so a secret stays a secret here too (kind, has_value).
--
-- NULL means "nothing set at this level", exactly as auth_json does next to it: the walk goes on to
-- the collection above and then to the environment, and an empty array would be a level that
-- answered "no variables" and stopped the walk.

ALTER TABLE collections ADD COLUMN variables_json TEXT;
