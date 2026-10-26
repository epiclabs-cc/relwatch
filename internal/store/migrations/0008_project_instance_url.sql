-- 0008: optional per-project provider instance URL (self-hosted GitLab/Gitea).
ALTER TABLE projects ADD COLUMN instance_url TEXT NOT NULL DEFAULT '';
