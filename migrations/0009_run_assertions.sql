-- What the scripts around a request of a run asserted, on the row of that request. The counts are
-- read from the scripting feature's reports when the row is made, and they are kept here rather than
-- looked up again: the report hangs off a record, and a run's row must still be able to say what was
-- asserted after that record has aged out of the history.

ALTER TABLE collection_run_results ADD COLUMN assertions_passed INTEGER NOT NULL DEFAULT 0;
ALTER TABLE collection_run_results ADD COLUMN assertions_total INTEGER NOT NULL DEFAULT 0;
