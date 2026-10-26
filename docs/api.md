# HTTP API

`relwatch serve` can expose a **JSON API** over the same SQLite database it
polls into. It reads and, with a token, **writes** the tracked projects, images,
and feeds. It is **disabled by default**.

Enabling the listener also serves the **web dashboard** at the same address (at
`/`); see [Web dashboard](web.md).

See [configuration](configuration.md) for enabling it; the short version:

```yaml
# settings.conf
http:
  enabled: true
  listen: "127.0.0.1:8080"
  token: "a-long-random-secret"   # optional; required off-loopback
```

or, per run: `relwatch serve --http 127.0.0.1:8080`.

- **Bind to loopback** (`127.0.0.1`) unless a reverse proxy fronts it. The API
  holds no TLS of its own.
- If `http.token` is set, **every** request must send
  `Authorization: Bearer <token>`; otherwise requests get `401`. With no token,
  loopback binding is the only protection — a warning is logged if you bind
  elsewhere without one.

The listening address is logged at startup:

```
http API listening on http://127.0.0.1:8080/api/v1
```

## Endpoints

All responses are JSON. Errors are `{"error": "..."}` with a matching status
code (`400` invalid input/id, `401` unauthorized, `404` not found, `405` wrong
method, `500` internal).

| Method | Path | Returns |
|---|---|---|
| `GET` | `/api/v1/status` | `{version, projects, images, feeds, last_checked_at, update}` |
| `GET` | `/api/v1/projects` | tracked projects + latest release |
| `POST` | `/api/v1/projects` | create a project (`{name, provider, repo, instance_url?}`) → `201` |
| `GET` | `/api/v1/projects/{id}` | one project's summary |
| `DELETE` | `/api/v1/projects/{id}` | delete a project → `204` |
| `GET` | `/api/v1/projects/{id}/releases` | that project's release history (newest first) |
| `GET` | `/api/v1/images` | tracked images + latest digest |
| `POST` | `/api/v1/images` | create an image (`{name, registry, repo}`) → `201` |
| `DELETE` | `/api/v1/images/{id}` | delete an image → `204` |
| `GET` | `/api/v1/images/{id}/digests` | that image's digest history (newest first) |
| `GET` | `/api/v1/feeds` | tracked feeds + latest release |
| `POST` | `/api/v1/feeds` | create a feed (`{name, url}`) → `201` |
| `GET` | `/api/v1/feeds/{id}` | one feed's summary |
| `DELETE` | `/api/v1/feeds/{id}` | delete a feed → `204` |
| `GET` | `/api/v1/feeds/{id}/releases` | that feed's release history (newest first) |

### Examples

```sh
curl -s http://127.0.0.1:8080/api/v1/status
# {"version":"v0.8.1","projects":3,"images":1,"feeds":2,
#  "last_checked_at":"2026-06-01T12:00:00Z",
#  "update":{"current":"v0.8.1","latest":"v0.9.0","update_available":true,"comparable":true}}

curl -s http://127.0.0.1:8080/api/v1/projects
# [{"id":1,"name":"svelte","provider":"github","repo":"sveltejs/svelte",
#   "created_at":"2026-06-01T00:00:00Z","latest_tag":"svelte@5.57.0",
#   "latest_released_at":"2026-05-30T12:00:00Z","latest_url":"https://..."}]

curl -s http://127.0.0.1:8080/api/v1/projects/1/releases
# [{"id":42,"project_id":1,"tag":"svelte@5.57.0","published_at":"...",
#   "url":"https://...","is_latest":true,"is_prerelease":false,"created_at":"..."}]

curl -s http://127.0.0.1:8080/api/v1/feeds
# [{"id":1,"name":"alpine","url":"https://alpinelinux.org/atom.xml",
#   "created_at":"2026-06-01T00:00:00Z","latest_tag":"2026.1",
#   "latest_released_at":"...","latest_url":"https://..."}]
```

With a token configured:

```sh
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/api/v1/status

# Create and delete a project
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"svelte","provider":"github","repo":"sveltejs/svelte"}' \
  http://127.0.0.1:8080/api/v1/projects
curl -s -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8080/api/v1/projects/1
```

## Exposing it remotely

Keep the API on loopback and put a TLS-terminating reverse proxy in front. Nginx:

```nginx
location /api/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    # The bearer token, if configured, is enforced by relwatch itself.
}
```

The systemd unit's sandbox (`ProtectSystem=strict`, `ReadWritePaths=...`) only
restricts the filesystem, so it does not affect the network listener; no unit
changes are needed to run the API.

## Stability

`/api/v1` is the stable surface; additive fields may appear, but existing fields
are not renamed within a major version. There is no pagination yet — the tracked
set is expected to be small.
