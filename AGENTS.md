# relwatch

Self-hosted CLI + daemon that polls Git providers (GitHub, GitLab, Gitea),
RSS/Atom feeds, and OCI container registries for new releases and emails a
digest of what's new. Written in Go; stores state in a single SQLite database.

## Tech Stack

- **Go 1.26** — single module, no framework.
- **Cobra** (`spf13/cobra`) — CLI;
- **robfig/cron** (`robfig/cron/v3`) — `serve` scheduling.
- **SQLite** (`modernc.org/sqlite`) — pure-Go driver, CGO-free static builds.
- Provider SDKs: `google/go-github`, `gitlab-org/api/client-go`, `code.gitea.io/sdk`.
- `encoding/xml` (stdlib) — Atom + RSS 2.0 parsing (no third-party feed library).
- `gopkg.in/yaml.v3` — `settings.conf`; `hashicorp/go-version` — semver comparison.
- `golang.org/x/crypto/bcrypt` + `golang.org/x/term` — dashboard auth.
- **No web framework** — stdlib `net/http` (Go 1.22 method+path mux) for the JSON
  API and a stdlib `html/template` dashboard, over a shared `internal/service`.
- **MkDocs Material** (CI-only) — renders `docs/` to GitHub Pages.

## Project Structure

```
├── cmd/relwatch/main.go           # entrypoint: version var + cli.NewRootCmd()
├── internal/
│   ├── cli/                       # cobra commands (root, paths, config, init, project/feed/image, check, serve, version, user)
│   ├── config/                    # settings.conf loader (db/smtp/http/update/schedule/prereleases)
│   ├── service/                   # domain layer shared by CLI, API, and web (reads + writes + auth)
│   ├── api/                       # JSON API (read + write), optional bearer token
│   ├── web/                       # server-rendered dashboard (html/template + embedded assets), session-auth'd
│   ├── scm/                       # provider seam: Provider{Name(); ListReleases()} + Factory.For(name, instanceURL)
│   ├── registry/                  # generic OCI Distribution v2 client (Factory.For(host))
│   ├── feed/                      # dependency-free Atom + RSS 2.0 parser
│   ├── notify/                    # SMTP digest delivery (STARTTLS / implicit TLS / none)
│   ├── update/                    # update-availability check against GitHub releases
│   └── store/                     # SQLite persistence + embedded migrations (migrations/NNNN_*.sql)
├── docs/                          # user docs (MkDocs, deployed to GitHub Pages)
├── docs-site/landing/index.html   # static landing page (mirror root; "moved to GitHub" notice)
├── contrib/relwatch.service       # systemd unit template (rendered by make install / installer)
├── install-relwatch.sh            # curl|bash installer (downloads from GitHub Releases)
├── Dockerfile                     # static binary → alpine, non-root
├── settings.conf.example          # config template (gitignored copy holds secrets)
├── .github/workflows/ci.yml       # test / release / docs
├── Makefile                       # build / release / test / vet / install / uninstall
└── LICENSE                        # MIT
```

## Database Schema

| Table | Key columns |
|---|---|
| `projects` | `id`, `name` (UNIQUE), `provider`, `repo`, `instance_url`, `created_at` — `UNIQUE(provider, repo)` |
| `releases` | `id`, `project_id` (→projects), `tag`, `name`, `published_at`, `url`, `is_latest`, `is_prerelease` — `UNIQUE(project_id, tag)` |
| `container_images` | `id`, `name`, `registry`, `repo`, `created_at` — `UNIQUE(name, registry, repo)` |
| `image_digests` | `id`, `image_id` (→container_images), `tag`, `digest`, `is_latest`, `observed_at` — `UNIQUE(image_id, tag)` |
| `feeds` | `id`, `name` (UNIQUE), `url` (UNIQUE), `created_at` |
| `feed_releases` | `id`, `feed_id` (→feeds), `tag`, `name`, `published_at`, `url`, `is_latest` — `UNIQUE(feed_id, tag)` |
| `meta` | `key` (PK), `value` |
| `users` | `id`, `email` (UNIQUE), `password_hash`, `created_at` |
| `sessions` | `id`, `token` (UNIQUE), `user_id` (→users), `csrf_token`, `created_at`, `expires_at` |
| `schema_migrations` | `id` (TEXT PK, filename), `applied_at` |

Timestamps are unix-epoch integers. Schema changes are new `migrations/NNNN_*.sql` files.

## Conventions

- `internal/scm` is the extension seam: add a provider by implementing
  `Provider{Name(); ListReleases()}` and registering it in `Factory.For`.
  `internal/registry` is the parallel seam for container registries.
- `internal/service` is the domain layer; both `internal/api` and `internal/web`
  go through it (never the store directly).
- SQL is hand-written (no ORM); store methods return `store.*` structs.
- No comments in code unless requested; errors wrapped (`fmt.Errorf("...: %w", err)`).
- Releases upsert by `(project_id, tag)`; a moved image digest is a new release.
- `serve` opens one store shared by the poll loop and the HTTP listener.

## Build / test / release

```bash
make build                          # static host binary -> bin/relwatch
make test                           # go test ./...
make vet                            # go vet ./...
make release VERSION=x.y.z          # static linux amd64/arm64 -> bin/release/
make version                        # preview the stamped version (dev, or semver from a v* tag)
```

## CI (`.github/workflows/ci.yml`)

- `test` — on push/PR: `go vet ./...` + `go test ./...`.
- `release` — on `v*` tags: `make release`, attach `relwatch-linux-{amd64,arm64}` +
  `SHA256SUMS` to the GitHub Release.
- `docs` — on `v*` tags: `mkdocs build --strict`, deploy to GitHub Pages.

Versioning: tag `v1.2.3` → binary reports `1.2.3` (leading `v` stripped); untagged
builds report `dev`.

## Environment variables

| Env | Default | Description |
|---|---|---|
| `RELWATCH_DB` | install-mode default | SQLite database path |
| `RELWATCH_CONFIG` | auto-discovered | Path to the YAML settings file |
| `RELWATCH_GITHUB_TOKEN` | empty | GitHub API token (rate limits) |
| `RELWATCH_GITLAB_TOKEN` | empty | GitLab API token (required for gitlab.com) |
| `RELWATCH_GITLAB_URL` | `https://gitlab.com/api/v4` | GitLab API base URL |
| `RELWATCH_GITEA_TOKEN` | empty | Gitea API token |
| `RELWATCH_GITEA_URL` | `https://gitea.com` | Gitea instance base URL |
| `RELWATCH_REGISTRY_TOKEN` | empty | OCI registry bearer token |
| `RELWATCH_REGISTRY_SCHEME` | `https` | OCI registry scheme |
| `RELWATCH_PRERELEASES` | `false` | Include pre-releases |
| `RELWATCH_SCHEDULE` | `0 * * * *` | Cron schedule for `serve` |
| `RELWATCH_HTTP_ADDR` | — | Bind address for the HTTP API/dashboard |
| `RELWATCH_UPDATE_SERVER` | GitHub `releases/latest` | Endpoint `serve` checks for updates |

## Notable behaviors

- **First check is a baseline** — history backfills silently; reporting starts on
  the second poll.
- **Updates are install-by-script** — no in-binary self-update; re-run
  `curl -fsSL https://raw.githubusercontent.com/epiclabs-cc/relwatch/main/install-relwatch.sh | sudo bash`.
  `serve` does a lightweight check of the GitHub releases once per poll and
  surfaces "a newer version is available" via stderr, the digest footer, and
  `/api/v1/status`.
- **Config/DB discovery** — config: `/etc/relwatch/settings.conf` → `./settings.conf`
  → `~/.config/relwatch/settings.conf`; DB: `--db` > `RELWATCH_DB` > `db:` > install
  default. `store.Open` uses WAL + `busy_timeout`, so a CLI and the running service
  can share one database.
- **Dashboard auth** — `relwatch user add EMAIL` bootstraps a user (bcrypt);
  `/login` issues an opaque session token (HttpOnly cookie); writes are CSRF-guarded.
