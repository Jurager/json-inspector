-- The environment a run went out under, kept with the run. A run is what happened, and what it
-- happened with is part of it: the page that reports on it is opened under whatever environment the
-- user is on now, and saying that about an old run would be saying something untrue about it.
--
-- A name and not an id: the environment may since have been renamed or deleted, and the run is not
-- changed by either. Runs from before this migration keep the empty string and are drawn without the
-- environment, which is what is known about them.

ALTER TABLE collection_runs ADD COLUMN environment TEXT NOT NULL DEFAULT '';
