-- Phases of a request, in microseconds and nullable.
--
-- Milliseconds were the wrong unit for a warm connection: on a reused one DNS, connect and TLS do
-- not happen at all, and even the body of a small response arrives in less than a millisecond. In
-- whole milliseconds every one of those read as 0, so the timings tab showed a live total and a row
-- of zeros that looked like a measurement that had failed.
--
-- Null is what "this did not happen" means: nothing was dialled because the connection was already
-- there, or nothing came back at all. Zero means a phase was measured and took no measurable time.

ALTER TABLE records ADD COLUMN duration_us INTEGER NOT NULL DEFAULT 0;
ALTER TABLE records ADD COLUMN dns_us INTEGER;
ALTER TABLE records ADD COLUMN connect_us INTEGER;
ALTER TABLE records ADD COLUMN tls_us INTEGER;
ALTER TABLE records ADD COLUMN wait_us INTEGER;
ALTER TABLE records ADD COLUMN download_us INTEGER;

-- A row written before this migration cannot tell "the phase took no time" from "the phase did not
-- happen": it had no way to say the second. A zero there is therefore not a measurement, and it
-- converts to NULL — the honest reading of a number that never carried information. A non-zero
-- phase is a real measurement and keeps its value.
UPDATE records
   SET duration_us = duration_ms * 1000,
       dns_us      = CASE WHEN dns_ms      > 0 THEN dns_ms * 1000 END,
       connect_us  = CASE WHEN connect_ms  > 0 THEN connect_ms * 1000 END,
       tls_us      = CASE WHEN tls_ms      > 0 THEN tls_ms * 1000 END,
       wait_us     = CASE WHEN wait_ms     > 0 THEN wait_ms * 1000 END,
       download_us = CASE WHEN download_ms > 0 THEN download_ms * 1000 END;

ALTER TABLE records DROP COLUMN duration_ms;
ALTER TABLE records DROP COLUMN dns_ms;
ALTER TABLE records DROP COLUMN connect_ms;
ALTER TABLE records DROP COLUMN tls_ms;
ALTER TABLE records DROP COLUMN wait_ms;
ALTER TABLE records DROP COLUMN download_ms;
