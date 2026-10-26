-- 0005: generic key/value metadata (single-value state, e.g. last_checked_at).
CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
