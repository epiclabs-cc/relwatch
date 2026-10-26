# Configuration

`relwatch` is configured through three layers, in order of increasing
precedence (later wins):

1. **`settings.conf`** — YAML file (database path, SMTP/email, schedule)
2. **Environment variables** — `RELWATCH_*`
3. **Command-line flags** — e.g. `--config`, `--db`, `--provider`

## Where the config and database live

The CLI keeps its state in one SQLite database and reads one `settings.conf`.
Both are found by convention, so you don't have to pass `--db`/`--config`
everywhere:

- A **systemd installation** uses `/etc/relwatch/settings.conf` +
  `/var/lib/relwatch/relwatch.db` and will create those on first install.
- A **standalone binary** uses `~/.config/relwatch/settings.conf` +
  `~/.local/share/relwatch/relwatch.db`. If no config exists yet, the first
  command that needs the database creates it (or run `relwatch init`).
- A **system install** (installer / `make install`) uses
  `/etc/relwatch/settings.conf` + `/var/lib/relwatch/relwatch.db`.

Run `relwatch config path` any time to see exactly which config and database
were resolved. See [Configuration](configuration.md) for the full precedence
rules.

`settings.conf` is discovered automatically (first match wins):

| Order | Location | Use |
|---|---|---|
| 1 | `--config PATH` | explicit override |
| 2 | `$RELWATCH_CONFIG` | explicit override |
| 3 | `/etc/relwatch/settings.conf` | system install (`install-relwatch.sh` / `make install`) |
| 4 | `./settings.conf` | local / dev checkout |
| 5 | `~/.config/relwatch/settings.conf` | per-user |

The database path resolves in this order:

1. `--db` flag
2. `RELWATCH_DB` env var
3. `db:` in `settings.conf`
4. install-mode default — **system** → `/var/lib/relwatch/relwatch.db`,
   **user** → `~/.local/share/relwatch/relwatch.db`, **local** →
   `./relwatch.db`

A relative `db:` resolves against the config file's directory and a leading `~`
is expanded. `relwatch config path` prints the resolved config + database.

When no `settings.conf` exists anywhere, the first command that needs the
database creates `~/.config/relwatch/settings.conf` (recording the per-user
`db:`). Use `relwatch init` to create it deliberately — it can also configure
SMTP interactively. A system install (installer / `make install`) instead writes
the DB path into `/etc/relwatch/settings.conf`.

## Settings file (`settings.conf`)

The settings file is discovered as above. Copy the committed example and edit it
for a local/dev setup:

```sh
cp settings.conf.example settings.conf
```

### Schema

```yaml
db: "/var/lib/relwatch/relwatch.db"
schedule: "0 * * * *"
smtp:
  enabled: true
  host: "smtp.example.com"
  port: 587
  tls: "auto"
  username: "relwatch@example.com"
  password: "smtp-app-password"
  from: "relwatch@example.com"
  to:
    - "you@example.com"
http:
  enabled: false
  listen: "127.0.0.1:8080"
  token: ""
update:
  server: "https://api.github.com/repos/epiclabs-cc/relwatch/releases/latest"
```

| Key | Type | Default | Meaning |
|---|---|---|---|
| `db` | string | install-mode default | SQLite database path (see above) |
| `smtp.enabled` | bool | `false` | Send a digest email when there are new releases |
| `smtp.host` | string | — | SMTP server hostname (required when `enabled`) |
| `smtp.port` | int | `587` | SMTP port (see `tls` below; `465` selects implicit TLS automatically) |
| `smtp.tls` | string | `auto` | Transport: `auto`, `starttls`, `implicit`, or `none` |
| `smtp.username` | string | — | SMTP AUTH username (omit for unauthenticated relays) |
| `smtp.password` | string | — | SMTP AUTH password |
| `smtp.from` | string | — | `From:` address (required when `enabled`) |
| `smtp.to` | list | — | `To:` recipients for the digest |
| `http.enabled` | bool | `false` | Serve the web dashboard and JSON API from `serve` |
| `http.listen` | string | `127.0.0.1:8080` | Bind address (keep on loopback behind a proxy) |
| `http.token` | string | — | Optional bearer token; enforced on every API request when set |
| `update.server` | string | `https://api.github.com/repos/epiclabs-cc/relwatch/releases/latest` | GitHub release endpoint checked by `serve` for available updates |

### `prereleases:` — include pre-releases

By default pre-releases are skipped. Set this to include them for `serve`:

```yaml
prereleases: true
```

Precedence: `--prereleases` flag > `RELWATCH_PRERELEASES` env > this key >
default (skip). See [Picking prereleases](#picking-prereleases).

### `schedule:` — serve interpolation clock

`serve` is a cron-driven poller; when to check is resolved at startup by the
`resolveSchedule` precedence chain (highest wins):

1. `--schedule` flag
2. `RELWATCH_SCHEDULE` env var
3. `schedule:` in `settings.conf` (this file)
4. built-in default `"0 * * * *"` (hourly)
5. other example `"0 6 * * 1"` (daily 06:00)

```yaml
schedule: "0 6 * * 1"    # weekly, Monday 06:00
```

Empty/absent `schedule:` in settings.conf simply falls through to the default —
it does not disable polling.

If `smtp.enabled` is `false` (or the file is absent), `serve` runs its checks
but sends no email — useful to try `serve` without any SMTP server yet.

### Notes on SMTP

`relwatch` speaks SMTP directly. `smtp.tls` chooses the transport:

- **`auto`** (default) — implicit TLS on port `465`, opportunistic STARTTLS
  otherwise. This covers the two common cases without thinking about it.
- **`starttls`** — connect in the clear, then upgrade via STARTTLS when the
  server advertises it (typical port `587`).
- **`implicit`** — TLS from the first byte (SMTPS, typically port `465`).
- **`none`** — no encryption; only for a trusted local relay.

PLAIN AUTH is used with `username`/`password` once the connection is encrypted.
No SMTP → `serve` still works; it just never sends mail. Keep `settings.conf`
**out of version control** — it contains credentials; track only
`settings.conf.example`.

### `http:` — web dashboard + JSON API

Optional. When `enabled: true`, `serve` also starts an HTTP listener over the
same database: the **web dashboard** at `/` (documented in [web.md](web.md)) and
the **JSON API** at `/api/v1` (documented in [api.md](api.md)). It is **disabled
by default**. Enable it either here:

```yaml
http:
  enabled: true
  listen: "127.0.0.1:8080"
  token: "a-long-random-secret"   # optional, but recommended off-loopback
```

or per-invocation with the `--http ADDR` flag / `RELWATCH_HTTP_ADDR` env var,
both of which enable the listener and set the bind address. Listen-address
precedence: `--http` flag > `RELWATCH_HTTP_ADDR` > `http.listen` > default
`127.0.0.1:8080`. `http.enabled` only decides whether it starts when no flag/env
address is given. Bind to loopback unless a reverse proxy fronts it; if a token
is set, every API request must send `Authorization: Bearer <token>`.

The dashboard has its own sign-in (see [web.md](web.md)); create the first user
with `relwatch user add EMAIL`. The API's `http.token` is a separate,
machine-facing credential.

### `update:` — update-check server

Optional. Points `serve`'s update check at the project's GitHub releases so it
can surface "a newer version is available" in its log, the digest footer, and
`/api/v1/status`. It defaults to the GitHub latest-release endpoint, so most
installs can omit it:

```yaml
update:
  server: "https://api.github.com/repos/epiclabs-cc/relwatch/releases/latest"
```

Precedence: `--update-server` flag > `RELWATCH_UPDATE_SERVER` env var >
`update.server` in `settings.conf` > built-in default. This is
notification-only: install/upgrade is done by re-running the installer
(`curl -fsSL https://raw.githubusercontent.com/epiclabs-cc/relwatch/main/install-relwatch.sh | sudo bash`).

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `RELWATCH_DB` | install-mode default | SQLite database path (overrides `db:`) |
| `RELWATCH_CONFIG` | auto-discovered | Path to the settings YAML file |
| `RELWATCH_GITHUB_TOKEN` | — | GitHub API token (`check` on github projects) |
| `RELWATCH_GITLAB_TOKEN` | — | GitLab API token (required for gitlab.com) |
| `RELWATCH_GITLAB_URL` | `https://gitlab.com/api/v4` | GitLab API base URL (self-hosted) |
| `RELWATCH_GITEA_TOKEN` | — | Gitea API token |
| `RELWATCH_GITEA_URL` | `https://gitea.com` | Gitea instance base URL |
| `RELWATCH_REGISTRY_TOKEN` | — | OCI registry bearer token (private images) |
| `RELWATCH_REGISTRY_SCHEME` | `https` | OCI registry scheme (`http` for self-hosted) |
| `RELWATCH_PRERELEASES` | `false` | Include pre-releases (`true`/`false`) |
| `RELWATCH_SCHEDULE` | `0 * * * *` | `serve` cron schedule |
| `RELWATCH_HTTP_ADDR` | — | Bind the dashboard/API here and enable it (overrides `http.listen`) |
| `RELWATCH_UPDATE_SERVER` | `https://api.github.com/repos/epiclabs-cc/relwatch/releases/latest` | GitHub release endpoint checked by `serve` for available updates (overrides `update.server`) |

## Command-line flags

Run `relwatch --help` at any time; the most common:

```sh
relwatch init [--non-interactive] [--db PATH]  # create a user settings.conf
relwatch config path                            # show resolved config + db
relwatch project add NAME OWNER/REPO --provider github|gitlab|gitea [--instance-url URL]
relwatch project remove ID|NAME|OWNER/REPO
relwatch feed add NAME URL
relwatch feed list
relwatch feed remove ID|NAME|URL
relwatch image add NAME HOST/REPO
relwatch image list
relwatch image remove ID|NAME|HOST/REPO
relwatch check [--project NAME] [--provider provider] [--prereleases]
relwatch serve [--config settings.conf] [--schedule "0 */12 * * *"] [--http 127.0.0.1:8080]
relwatch version
```

`--provider` on `check` filters tracked projects to one provider (`github`,
`gitlab`, or `gitea`). `--project` can be a numeric ID, an exact project name,
or an exact `OWNER/REPO`; when a name or repo is ambiguous (tracked on several
providers), pass `--provider` to disambiguate. Feeds are checked automatically
and are not subject to `--project`/`--provider` filtering. Registry flags
(`--registry-token`, `--registry-scheme`) are global and apply to every image in
a `check`. See [Container images](providers.md) for the image commands.

### Picking prereleases

Pre-releases are skipped by default; the precedence is:

1. `--prereleases` flag
2. `RELWATCH_PRERELEASES` env var
3. `prereleases:` in `settings.conf`
4. default: skip
