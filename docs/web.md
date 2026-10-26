# Web dashboard

`relwatch serve` can serve a **web dashboard** alongside the JSON API. It
shows and manages the tracked data — projects, images, and feeds — in a minimal,
server-rendered interface.

## Enable it

The dashboard is served on the same listener as the JSON API, so enabling one
enables both:

```sh
relwatch serve --http 127.0.0.1:8080
```

Then open `http://127.0.0.1:8080/` for the dashboard (the API remains under
`/api/v1`). See [Configuration](configuration.md) for `http.enabled` /
`http.listen` and the `RELWATCH_HTTP_ADDR` env var.

## Auth

The dashboard requires sign-in. Create the first user from the CLI:

```sh
relwatch user add admin@example.com
```

`user add` prompts for a password (or pass `--password` non-interactively), and
`user list` shows existing users. Each user signs in at `/login` and gets a
session cookie (HttpOnly, 30 days); signing out drops the session.

The JSON API keeps its own optional `http.token` bearer auth, independent of the
dashboard's sessions. Both bind to loopback by default — put a TLS-terminating
reverse proxy in front of anything public.

## Pages

| Path | Shows |
|---|---|
| `/` | Overview with counts of tracked projects, images, feeds, and last-checked time |
| `/projects` | Projects and their latest release |
| `/projects/{id}` | One project's release history |
| `/images` | Images and their latest tag/digest |
| `/images/{id}` | One image's digest history |
| `/feeds` | Feeds and their latest release |
| `/feeds/{id}` | One feed's release history |

The overview's "last checked" timestamp is recorded on every successful `check`
or `serve` poll, and is also reported as `last_checked_at` by `/api/v1/status`.

## Managing entries

Each of `/projects`, `/images`, and `/feeds` has an **add** form at the top and a
**Remove** button per row, so a signed-in user can create and delete entries from
the browser (no CLI needed). Write actions are protected by a per-session CSRF
token.
