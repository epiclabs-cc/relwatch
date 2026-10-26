<p align="center">
  <img src="docs/assets/logo-light.jpg" alt="RelWatch" width="128">
</p>

# RelWatch

Poll your GitHub, GitLab, and Gitea projects — plus your container images and
RSS/Atom feeds — on a schedule and get an email digest whenever there's a new
release.

[![CI](https://github.com/epiclabs-cc/relwatch/actions/workflows/ci.yml/badge.svg)](https://github.com/epiclabs-cc/relwatch/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

RelWatch is a lightweight, self-hosted binary that consolidates release
tracking from multiple platforms, eliminating the need to manually check each
source individually.

- **Silent by default**: the first `check` of a project (or image) backfills its
  history quietly; only releases that appear *after* you start watching get
  reported.
- **`serve`** runs an immediate check, then polls on a cron schedule and emails
  a digest listing any new releases to your configured recipients.

## Install

Prebuilt Linux binaries are published on
[GitHub Releases](https://github.com/epiclabs-cc/relwatch/releases/latest).
Deploy as a systemd service with one command (creates the service user,
installs the hardened unit, and enables the scheduled poll):

```sh
curl -fsSL https://raw.githubusercontent.com/epiclabs-cc/relwatch/main/install-relwatch.sh | sudo bash
```

Or from a checkout: `sudo ./install-relwatch.sh`. See
[Deployment](docs/deployment.md) for `make install`, Docker, and a plain binary.

## Quick start

```sh
# Track a project, a feed, and a container image
relwatch project add cli cli/cli --provider github
relwatch feed add alpine https://alpinelinux.org/atom.xml
relwatch image add alpine docker.io/library/alpine

# Check for new releases now
relwatch check

# Poll every hour and email a digest of anything new
relwatch serve
```

State lives in one SQLite database and one `settings.conf`, found by
convention: a system install uses `/etc/relwatch/settings.conf` +
`/var/lib/relwatch/relwatch.db`, while a standalone binary uses
`~/.config/relwatch/settings.conf` + `~/.local/share/relwatch/`. Run
`relwatch config path` to see what was resolved, or `relwatch init` to
create a per-user config.

## Features

- **Multiple providers**: GitHub, GitLab, Gitea (and more via `internal/scm`).
- **RSS/Atom feeds**: a first-class feature (`feed add NAME <feed-url>`) alongside
  Git repos and images.
- **Container images**: track tags across any OCI Distribution v2 registry
  (Docker Hub, GHCR, GCR, Quay, `registry.gitlab.com`, self-hosted).
- **SQLite persistence**: releases are upserted by `(project_id, tag)` and image
  digests by `(image_id, tag)`; an existing entry is updated, never re-reported
  as new.
- **First-check baseline**: `check` backfills silently; reporting starts on the
  second poll.
- **Prerelease opt-in**: skip prereleases by default; include them with
  `--prereleases` / `RELWATCH_PRERELEASES` / `settings.conf`.
- **Email digest**: `serve` with SMTP configured sends one digest email per run
  when there are new releases (STARTTLS on 587, implicit TLS on 465).
- **JSON API**: `serve` can expose your projects/images/feeds over HTTP
  (`--http 127.0.0.1:8080`, optional bearer token) for dashboards and scripts,
  with write endpoints for managing entries.
- **Web dashboard**: a server-rendered dashboard at the same address (`/`) as
  the API — view and manage projects, images, and feeds, with per-user sign-in.
- **Systemd installer**: `install-relwatch.sh` provisions a hardened service.
- **Update notifications**: `serve` checks the GitHub releases once per poll and
  surfaces an available update in its log, the digest footer, and `/api/v1/status`.
- **Static builds**: `make release VERSION=x.y.z` produces static Linux
  amd64/arm64 binaries stamped with the version.

## Docs

Published at **<https://epiclabs-cc.github.io/relwatch/>** (rebuilt on each
release) and kept here as Markdown:

- [Getting started](docs/getting-started.md) — install, first project/image, `check`, `serve`.
- [Configuration](docs/configuration.md) — config/db discovery, `settings.conf` (`db:` + `smtp:` + `http:` + `update:` + `schedule:`), env vars, flags.
- [HTTP API](docs/api.md) — JSON endpoints (read + write), auth, reverse-proxy.
- [Web dashboard](docs/web.md) — browser UI, user sign-in, management.
- [Container images](docs/providers.md) — track OCI registry tags/digests.
- [Deployment](docs/deployment.md) — `install-relwatch.sh`, `make install`, Docker, plain binary.
- [Notifications](docs/notifications.md) — digest content, scheduling, SMTP setup.

## License

[MIT](LICENSE).
