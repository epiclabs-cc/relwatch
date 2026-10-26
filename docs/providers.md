# Providers how-to

Providers are the different types of sources you can track using relwatch. The currently available provider types are:

* Projects (Git)
* Container images (OCI registries)
* Feeds (RSS/Atom feeds)

The following sections include some commonly used commands to manage providers and how to manage projects, images and feeds.

## Projects (Git)

## Container images (OCI registries)

`relwatch` tracks container image tags alongside Git releases. Instead of
polling a provider's release API, it talks the **OCI Distribution v2** protocol
directly, so a single implementation covers Docker Hub, GHCR, GCR, Quay,
`registry.gitlab.com`, and any self-hosted registry.

### Track an image

An image is identified by `HOST/REPO` (the registry host, then the repository
path). The first argument is your short `NAME` label, exactly like
`project add`:

```sh
relwatch image add alpine docker.io/library/alpine
relwatch image add grafana ghcr.io/grafana/grafana
relwatch image add caddy ghcr.io/caddyserver/caddy
relwatch image add node-exporter quay.io/prometheus/node-exporter
```

The `HOST/REPO` string is split on the **first** `/`: everything before it is
the registry host, the rest is the repository path. Docker Hub's official
images live under the `library/` namespace, so use `docker.io/library/alpine`,
not `docker.io/alpine`.

**Verify it landed:**

```sh
relwatch image list
```

```
ID  NAME           REGISTRY                   REPO                  LATEST TAG  LATEST DIGEST
1   alpine         docker.io                  library/alpine        3.20        sha256:...
2   grafana        ghcr.io                    grafana/grafana       latest      sha256:...
```

`LATEST TAG` / `LATEST DIGEST` are `(not checked yet)` until the first `check`.

### Check images

`check` polls tracked projects **and** tracked images in the same run — there is
no separate command:

```sh
relwatch check
```

For each tracked image it lists the repository's tags and resolves each tag to
its manifest **digest**. The semantics mirror Git releases:

- **The first check is a baseline.** Digests are recorded silently; nothing is
  reported. Reporting starts from the second check onward.
- **A moved tag is a new release.** If a tracked tag's digest changes (e.g.
  `latest` or `3.20` is republished), that is treated as a new release — a
  registry is often the only place a re-pushed tag is visible.
- Tags that cannot be resolved (e.g. an architecture-specific manifest the
  registry can't serve) are skipped, not fatal.

New image releases appear in the `serve` **email digest** with the provider
label `registry`:

```
alpine - registry/docker.io/library/alpine: 3.20
https://docker.io/v2/library/alpine/manifests/3.20
```

See [Notifications](notifications.md) for the digest format.

### Remove an image

```sh
relwatch image remove alpine                 # by NAME
relwatch image remove docker.io/library/alpine  # by HOST/REPO
relwatch image remove 2                      # by numeric ID
```

Removing an image deletes its digest history too (the `image_digests` rows
cascade).

### Other registries

#### Private registries

Public images work anonymously — the client transparently follows the
registry's `WWW-Authenticate` Bearer challenge. For a private repository, supply
a token:

```sh
relwatch check --registry-token "$REGISTRY_TOKEN"
export RELWATCH_REGISTRY_TOKEN="$REGISTRY_TOKEN"
```

The token is a standard registry bearer/API token (for GHCR a personal access
token with `read:packages`, for Docker Hub a PAT, etc.). Note that GitHub
Container Registry packages are **private by default**: anonymous pulls work
only for packages the owner has explicitly marked public, so a GHCR image that
fails anonymously usually just needs a token. Precedence:

1. `--registry-token` flag
2. `RELWATCH_REGISTRY_TOKEN` env var
3. anonymous

#### Self-hosted / plain HTTP

Registries are spoken over `https` by default. For a local or TLS-terminated
registry on plain HTTP:

```sh
relwatch check --registry-scheme http
# or: export RELWATCH_REGISTRY_SCHEME=http
```

## Feeds (RSS/Atom)

