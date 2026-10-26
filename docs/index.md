# Introduction

**RelWatch monitors code repositories and container images for new releases,
delivering convenient email digests so you never miss an important update.**

It is a lightweight, self-hosted binary that consolidates release tracking from
multiple platforms — GitHub, GitLab, Gitea, OCI container registries, and
RSS/Atom feeds — eliminating the need to manually check each source individually.

## What you get

- **One place to watch everything** — Git releases, container image tags, and
  feeds, tracked in a single SQLite database and polled on one schedule.
- **Silent by default** — the first `check` of a project (or image) backfills its
  history quietly; only releases that appear *after* you start watching are
  reported.
- **Email digests** — `serve` runs an immediate check, then polls on a cron
  schedule and emails a digest listing any new releases to your configured
  recipients.
- **Self-hosted and lightweight** — a single static binary and one SQLite
  database; no external service or account required.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/epiclabs-cc/relwatch/main/install-relwatch.sh | sudo bash
```

Prebuilt binaries are published on
[GitHub Releases](https://github.com/epiclabs-cc/relwatch/releases/latest)
(verified against the release's `SHA256SUMS` on install).

## Start here

- [Getting started](getting-started.md) — install, track your first project,
  image, and feed, then run `check` and `serve`.
- [Configuration](configuration.md) — config/database discovery, `settings.conf`,
  environment variables, and flags.
- [Notifications](notifications.md) — digest contents and SMTP setup.
- [Container images](providers.md) — track OCI registry tags and digests.
- [HTTP API](api.md) — the JSON API served by `serve`.
- [Web dashboard](web.md) — the browser dashboard and user sign-in.
- [Deployment](deployment.md) — installer, `make install`, Docker, plain binary.
