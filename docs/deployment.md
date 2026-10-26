# Deployment

`relwatch` ships as a single static binary and a matching set of ops
artifacts so you can run it the way your infra prefers. A system install
(installer or `make install`) uses the standard FHS split — **binary** under the
prefix, **config** under `/etc`, **state** under `/var/lib`:

| Path | Purpose |
|---|---|
| `/opt/relwatch/relwatch` | the binary (prefix defaults to `/opt/relwatch`) |
| `/etc/relwatch/settings.conf` | your config (SMTP password + `db:`); **never overwritten** |
| `/var/lib/relwatch/relwatch.db` | SQLite poll state (writable by the service user) |
| `/etc/systemd/system/relwatch.service` | the systemd unit |

Choose one:

1. **Systemd via the installer** (`install-relwatch.sh`) — recommended; one
   command on a VM or root server.
2. **Systemd from a checkout** (`make install`) — same layout when you build
   from source.
3. **Docker** (`Dockerfile`) — `serve` in a non-root container on any platform.
4. **Plain binary** — copy the release artifact to `PATH` and run `serve`.

For the general rules the CLI uses to pick a config and database (including
per-user `~/.config` and `~/.local/share` locations), see
[configuration](configuration.md).

## Installation

### Systemd installer

`install-relwatch.sh` downloads the latest release binary from GitHub Releases,
creates the unprivileged `relwatch` system user, installs the hardened unit,
seeds `/etc/relwatch/settings.conf`, and enables the scheduled poll:

```sh
curl -fsSL https://raw.githubusercontent.com/epiclabs-cc/relwatch/main/install-relwatch.sh | sudo bash
```

Or from a checkout: `sudo ./install-relwatch.sh` (add `--dl-url URL` to pull
binaries from a different host).

Useful options (see `--help`):

| Option | Effect |
|---|---|
| `--dl-url URL` | GitHub Releases base URL (default `https://github.com/epiclabs-cc/relwatch/releases`) |
| `--tag TAG` | install a specific release instead of `latest` |
| `--prefix DIR` | prefix for the **binary** (default `/opt/relwatch`; config/state stay in `/etc` + `/var/lib`) |
| `--schedule CRON` | poll schedule (default `0 * * * *`) |
| `--smtp-host HOST` … | non-interactive SMTP setup (see below) |
| `--no-enable` | install the unit but don't start/enable it |
| `--uninstall` | stop + remove the service (keeps `/etc/relwatch/settings.conf` + `/var/lib/relwatch`) |

The installer verifies each download is a Linux ELF and checks it against the
release's `SHA256SUMS` before installing, so an HTML error page can never be
installed as the binary.

On a terminal the installer prompts for the SMTP server and digest recipient(s)
and writes `/etc/relwatch/settings.conf` (recording `db:
/var/lib/relwatch/relwatch.db`). For automation, pass the values as flags
(`--smtp-host`, `--smtp-port`, `--smtp-user`, `--smtp-pass`, `--smtp-from`,
`--smtp-to`, `--smtp-tls`) or use `--non-interactive` to accept the examples and
edit the file later.

After install:

```sh
systemctl status relwatch
journalctl -u relwatch -f
sudoedit /etc/relwatch/settings.conf   # then: systemctl restart relwatch

# Manage the service's tracked projects (run as the service user so the
# database stays writable by the service):
sudo -u relwatch /opt/relwatch/relwatch project add <name> <owner/repo>
sudo -u relwatch /opt/relwatch/relwatch project list
```

Because the config lives at `/etc/relwatch/settings.conf`, a manual
`relwatch` invocation on the host resolves to the same config and database
the service uses — no `--db` needed. Confirm with `relwatch config path`.

#### The unit's sandbox

The unit runs as the unprivileged `relwatch` system user with a hardened
sandbox so the strictly-outbound `serve` process can send SMTP mail but touch
little else:

- `NoNewPrivileges`, `PrivateTmp`, `PrivateDevices`;
- `ProtectSystem=strict`, `ProtectHome=yes`, `ProtectKernel{Tunables,Modules}`;
- an empty `CapabilityBoundingSet` and `SystemCallFilter=@system-service`;
- **`ReadWritePaths=/var/lib/relwatch`** — the one writable path, so SQLite can
  keep its poll state under `ProtectSystem=strict`.

#### Scheduling checks

To set up a polling schedule, see [configuration](configuration.md) for the full table.

### Docker

```sh
docker build -t relwatch .
docker run -d \
  --name relwatch \
  -v "$(pwd)/settings.conf:/etc/relwatch/settings.conf:ro" \
  -v relwatch-db:/var/lib/relwatch \
  relwatch
```

- Multi-stage build: a `golang` stage compiles the static binary with the same
  `-X` version ldflag as the release Makefile, then the runtime stage is a
  minimal `alpine` image running as non-root (`relwatch` user, uid/gid 10001
  unless overridden with `--build-arg RELWATCH_UID/GID`).
- The image contains no HTTP listener — `serve` is cron + SMTP-outbound only,
  so nothing to expose.
- **Mount your `settings.conf` at `/etc/relwatch/settings.conf`** (it holds
  the SMTP password and is gitignored); the example at
  `/etc/relwatch/settings.conf.example` exists only to guide a first boot.
  The entrypoint quietly adopts the mounted file.
- Persist the DB in a volume at `/var/lib/relwatch`.

`schedule` precedence is identical to the systemd path (flag > env >
`settings.conf schedule:` > hourly default).

### Plain binary

Download the release artifact from GitHub Releases and run it:

```sh
curl -fsSLO https://github.com/epiclabs-cc/relwatch/releases/latest/download/relwatch-linux-amd64
chmod +x relwatch-linux-amd64
./relwatch-linux-amd64 init        # creates ~/.config/relwatch/settings.conf
./relwatch-linux-amd64 serve
```

With no config present, the CLI creates `~/.config/relwatch/settings.conf`
and a per-user database under `~/.local/share/relwatch/`; `init` does this
deliberately and can configure SMTP interactively. `relwatch config path`
prints the resolved paths.

The mirror serves a stable `latest/` plus versioned `vX.Y.Z/` directories, each
with a `SHA256SUMS`.

## Web dashboard + HTTP API

`serve` can expose the web dashboard and a JSON API (see [web.md](web.md) and
[api.md](api.md)). It is disabled by default; enable it in
`/etc/relwatch/settings.conf`:

```yaml
http:
  enabled: true
  listen: "127.0.0.1:8080"
  token: "a-long-random-secret"   # recommended if not loopback-only
```

```sh
sudo systemctl restart relwatch
journalctl -u relwatch | grep "http API"   # http API listening on http://127.0.0.1:8080/api/v1
```

The systemd sandbox restricts only the filesystem (`ProtectSystem=strict` +
`ReadWritePaths=/var/lib/relwatch`), so listening on a port needs **no unit
changes**. For remote access, keep it on loopback and reach it through an SSH
tunnel (`ssh -L 8080:127.0.0.1:8080 user@host`) or a TLS reverse proxy. The
config file is read at startup, so changing `http.*` requires a restart.

## Uninstall

`sudo ./install-relwatch.sh --uninstall` (or `sudo make uninstall`) stops and
disables the unit, removes the binary and the unit file, and reloads systemd —
while **keeping** `/etc/relwatch/settings.conf` and `/var/lib/relwatch` so
a re-install is quick and your SMTP password and poll history are preserved. The
installer also leaves the `relwatch` system user in place (remove it with
`userdel relwatch`). For Docker, `docker rm -f relwatch` and drop the
named volume.
