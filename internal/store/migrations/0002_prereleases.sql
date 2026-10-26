-- 0002: prerelease awareness + removed-project bookkeeping.
ALTER TABLE releases ADD COLUMN is_prerelease INTEGER NOT NULL DEFAULT 0;
