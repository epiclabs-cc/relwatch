-- 0004: feeds (separate from git projects)
CREATE TABLE feeds (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    url        TEXT    NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE (name),
    UNIQUE (url)
);

CREATE TABLE feed_releases (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id      INTEGER NOT NULL REFERENCES feeds (id) ON DELETE CASCADE,
    tag          TEXT    NOT NULL,
    name         TEXT    NOT NULL DEFAULT '',
    published_at INTEGER,
    url          TEXT    NOT NULL DEFAULT '',
    is_latest    INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE (feed_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_feed_releases_feed_id ON feed_releases (feed_id);
CREATE INDEX IF NOT EXISTS idx_feed_releases_published ON feed_releases (feed_id, published_at DESC);
