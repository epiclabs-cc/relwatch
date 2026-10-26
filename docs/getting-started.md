# Getting started

The walkthrough in this document covers how to get started with relwatch in the easiest possible way. At the end of the page it also includes a brief summary of some the failure modes you're most likely to hit on the first run. It assumes you have the binary in your `PATH` — for more details, see the [Installation](#installation) page.

## Installation

On a Linux system using systemd, be it a bare metal host, a VM or an LXC container, you can install and configure relwatch in about 5 minutes. 

### On a server (systemd)

The one-shot installer downloads the latest release binary, creates the
unprivileged `relwatch` system user, installs the hardened unit, and enables
the scheduled poll:

```sh
curl -fsSL https://raw.githubusercontent.com/epiclabs-cc/relwatch/main/install-relwatch.sh | sudo bash
```

Prebuilt binaries are published on
[GitHub Releases](https://github.com/epiclabs-cc/relwatch/releases/latest),
which needs no token.

### Prebuilt binaries

Download `relwatch-linux-amd64` or `relwatch-linux-arm64` from
[GitHub Releases][releases], then:

```sh
curl -fsSLO https://github.com/epiclabs-cc/relwatch/releases/latest/download/relwatch-linux-amd64
chmod +x relwatch-linux-amd64
sudo mv relwatch-linux-amd64 /usr/local/bin/relwatch
relwatch init          # optional: create ~/.config/relwatch/settings.conf now
```

## Basic operations

To get started tracking projects, follow the basic operations commands below.

### Track projects (Git)

```sh
relwatch project add name owner/repo --provider github
```

* `OWNER/REPO` is the repository's owner and repo as it appears in the provider
(no leading `https://`).
* `NAME` (name) is the short label you'll use in `check` —
it can be anything, but pick something you'd type every day.

Supported providers: `github`, `gitlab`, and `gitea`. Repositories whose name
matches on more than one provider? That's fine — the name is your own namespace; the
provider is stored with the project.

To verify that the project was added, run:

```sh
relwatch project list
```

To stop tracking a project (and drop its release history), use
`relwatch project remove NAME` — or a numeric ID, or `OWNER/REPO`.

#### Check for new releases

```sh
relwatch check --project name
```

The very first `check` of a project is a **baseline**: it backfills the release
history silently and reports nothing. From the **second** check onward, only
releases that appeared since the previous poll are reported as new. This means
you can wire `check` into your own cron/etc without spamming yourself with
everything the repo released last year.

#### Picking a single project

`--project` accepts a numeric ID, an exact project name, or an exact
`OWNER/REPO`:

```sh
relwatch check --project 3            # numeric = project ID
relwatch check --project name         # by name
relwatch check --project org/repo     # by repo
```

If the same repo is tracked on several providers and you don't disambiguate,
`check` errors and lists the candidates. Add `--provider` to pick one:

```sh
relwatch check --project name --provider github
```

Prereleases are **skipped by default**. Include them for a run (or persistently)
with:

```sh
relwatch check --prereleases
export RELWATCH_PRERELEASES=true
```

### Track feeds (RSS)

Not every project publishes releases through a Git provider or a registry — some
announce them on an RSS/Atom feed. Feeds are a first-class feature of their own,
tracked with `feed add NAME URL`:

```sh
relwatch feed add alpine https://alpinelinux.org/atom.xml
relwatch feed list
relwatch feed remove alpine
```

Each feed entry becomes a release: the entry's `guid`/`id` is the tag, the title
the release name, and the link the release URL. `check` and the email digest
treat feed items like any other release. Both **Atom** and **RSS 2.0** feeds are
supported; feeds have no prerelease concept, so entries are never marked
prerelease.

### Track container images (OCI containers)

Images work the same way as projects — `image add NAME HOST/REPO`, where
`HOST/REPO` names an OCI registry repository:

```sh
relwatch image add alpine docker.io/library/alpine
relwatch image add grafana ghcr.io/grafana/grafana
relwatch image list
```

`check` polls images alongside projects, and a tag whose **digest** changes is
reported as a new release (so `latest`/`3.20` being republished is caught). See
[Container images](providers.md) for private registries and self-hosted setups.


## Poll on a schedule

```sh
relwatch serve
```

`serve` runs one check immediately, then polls on a cron schedule (default
`0 * * * *`, i.e. hourly) forever. Add `--schedule "0 */12 * * *"` for
twice-daily, or set `RELWATCH_SCHEDULE`.

To get **email digests** of new releases, configure SMTP — see
[Configuration](configuration.md).

To view a **web dashboard** of everything you track, create a user and start
`serve` with HTTP enabled:

```sh
relwatch user add you@example.com
relwatch serve --http 127.0.0.1:8080
```

Then open `http://127.0.0.1:8080/` and sign in — see [Web dashboard](web.md).
From the dashboard you can view and manage (add/remove) your projects, images,
and feeds in the browser.


## What happens when things go wrong

- **First check reports nothing** — expected; it's the baseline. Check again.
- **A token is missing for a provider** — `check` for that provider errors
  with a message naming the missing env var (e.g. `RELWATCH_GITHUB_TOKEN`).
- **A provider is unreachable** — that project is skipped for the run; the
  error goes to stderr, other projects still get checked. One failure doesn't
  abort the run.
- **A release is re-reported** — releases are upserted by
  `(project, tag)`; only genuinely first-seen `tag`s are new.

[releases]: https://github.com/epiclabs-cc/relwatch/releases/latest
