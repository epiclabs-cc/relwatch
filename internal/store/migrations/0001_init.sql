CREATE TABLE IF NOT EXISTS projects (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    provider   TEXT    NOT NULL DEFAULT 'github',
    repo       TEXT    NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE (name),
    UNIQUE (provider, repo)
);

CREATE TABLE IF NOT EXISTS releases (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id   INTEGER NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    tag          TEXT    NOT NULL,
    name         TEXT    NOT NULL DEFAULT '',
    published_at INTEGER,
    url          TEXT    NOT NULL DEFAULT '',
    is_latest    INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE (project_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_releases_project_id ON releases (project_id);