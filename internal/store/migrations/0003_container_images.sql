-- 0003: container image (OCI Distribution v2) tracking.
-- A moved tag (digest change) is treated as a new release (v0.4.0 Q3).
CREATE TABLE container_images (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    registry   TEXT NOT NULL,
    repo       TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE UNIQUE INDEX idx_container_images_lookup ON container_images (name, registry, repo);

CREATE TABLE image_digests (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    image_id    INTEGER NOT NULL REFERENCES container_images(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    digest      TEXT NOT NULL,
    is_latest   INTEGER NOT NULL DEFAULT 0,
    observed_at INTEGER NOT NULL,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE (image_id, tag)
);
CREATE INDEX idx_image_digests_observed ON image_digests (image_id, observed_at DESC);
