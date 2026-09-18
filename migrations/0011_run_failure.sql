-- A run's row carries the refusal behind its error when the app is the one that said no: a request
-- whose `{{tokens}}` answered to nothing, say. The code and the values its sentence needs travel as
-- JSON, because the window is what words a refusal, and Go's own account of one is machine text
-- (`variableMissing(n=1, names=var3)`) that was never meant to be read.
--
-- It is stored rather than derived from the error text: a run read back a week later has to say what
-- the window said while the run was going, and a refusal is not a thing to parse back out of a
-- string. An empty value is every row the app did not refuse — the network, a socket, a timeout.
ALTER TABLE collection_run_results ADD COLUMN failure TEXT NOT NULL DEFAULT '';
