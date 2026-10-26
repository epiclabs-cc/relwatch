-- 0007: per-session CSRF token for dashboard write forms.
ALTER TABLE sessions ADD COLUMN csrf_token TEXT NOT NULL DEFAULT '';
