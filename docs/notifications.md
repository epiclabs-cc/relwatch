# Notifications

`relwatch` can email you a **digest** of new releases on a schedule. This
page describes what gets reported, when, and how to wire up SMTP.

## What triggers email

`serve` does two things:

1. Runs a `check` immediately at startup.
2. Then polls on a cron schedule (default hourly: `0 * * * *`).

After each check, if SMTP is enabled **and** at least one new release was found,
`relwatch` sends **one digest email** listing the new releases for that run.
If nothing new appeared, no email is sent.

## What the digest contains

One email per run. Subject:

```
relwatch: N new release(s)
```

Body lists each new release on its own line:

```
PROJECT - PROVIDER/REPO: TAG
```

with the release `URL` on the next line, and the publish date when available.
Container-image releases are included too, with the provider shown as
`registry` and the digest as the release identity:

```
alpine - registry/docker.io/library/alpine: 3.20
https://docker.io/v2/library/alpine/manifests/3.20
```

Releases are reported **once**: a project release is keyed by `(project_id,
tag)` and an image tag by `(image_id, tag)`, so an already-seen entry is
updated, never re-reported as new. For images, a tag whose **digest** changes
counts as a new release.

## First check is silent

The very first `check` of a project **backfills** its release history silently —
it's a baseline. Only releases that appear *after* that baseline (i.e. from the
second check onward) are reported as new. The same applies to the first check of
a container image. So you won't get a wall of everything the project released
years ago; reporting starts fresh from when you start watching.

## Scheduling

The default cron schedule is `0 * * * *` (every hour, on the hour). Change it
with:

```sh
relwatch serve --schedule "0 */12 * * *"      # twice a day
relwatch serve --schedule "0 9 * * 1-5"       # weekdays at 9am
```

or via `RELWATCH_SCHEDULE`.

## SMTP setup

Create `settings.conf` (copy `settings.conf.example`) with the `smtp` block:

```yaml
smtp:
  enabled: true
  host: "smtp.example.com"
  port: 587
  tls: "auto"
  username: "relwatch@example.com"
  password: "your-smtp-password"
  from: "relwatch@example.com"
  to:
    - "you@example.com"
```

- `host:port` — the SMTP server. `tls: auto` (the default) uses implicit TLS
  on port `465` and opportunistic STARTTLS otherwise, with PLAIN AUTH once the
  connection is encrypted. Force a mode with `tls: starttls|implicit|none`.
- `to` — each recipient gets one copy of the digest.
- No SMTP server handy? Leave `enabled: false` (or no file) — `serve` runs fine
  and simply doesn't send mail until you configure one.
- **Never commit `settings.conf`** — it holds credentials. Only the
  `settings.conf.example` template is tracked.

## Troubleshooting

- **No email, but `serve` is running** — SMTP isn't enabled, or there were no
  new releases that run (check stderr: `serve` logs the check/digest to
  stderr, and errors never abort the whole run).
- **Digest empty every run** — you're still on the first (baseline) check of a
  fresh project. Run it again after something is released.
- **Auth rejected (535)** — check `username`/`password`; some relays need an
  app-specific password rather than your login password.
