-- The account: one row for the whole installation, not a row per something.
--
-- It is not a "setting", because it is a state rather than a choice, and not a "session", because the
-- app has one session: signed in is signed in. That is where the CHECK on the single row comes from —
-- two rows are two answers to "who am I", and one of them is always wrong.
--
-- The refresh token sits here as it is, and it is the same trade the environments' secrets are: SQLite
-- carries them in plain text, the file is closed to 0600, and the interface says so. It cannot be kept
-- as a hash — it is a credential to use, not one to check against.
CREATE TABLE account (
    id            INTEGER PRIMARY KEY CHECK (id = 1),
    -- The address of the server, which is also the issuer of its tokens. Kept even while nobody is
    -- signed in: the person chose it, and there is nothing to ask a second time.
    server        TEXT    NOT NULL DEFAULT '',
    user_id       TEXT    NOT NULL DEFAULT '',
    email         TEXT    NOT NULL DEFAULT '',
    name          TEXT    NOT NULL DEFAULT '',
    -- What the account is entitled to, as the server answered. An empty string is "nobody asked".
    plan          TEXT    NOT NULL DEFAULT '',
    seats         INTEGER NOT NULL DEFAULT 0,
    -- This device's session on the server: it is what makes one row of the device list recognisable as
    -- "this device", and it is what the server revokes a sign-in with.
    session_id    TEXT    NOT NULL DEFAULT '',
    refresh_token TEXT    NOT NULL DEFAULT '',
    signed_in_at  INTEGER NOT NULL DEFAULT 0
);
