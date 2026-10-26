# relwatch — build a static binary, then run it as a non-root user.
#
# The container layout mirrors a `make install` / systemd install so a Docker
# deploy and a systemd install behave identically:
#   /opt/relwatch/relwatch          # the binary
#   /etc/relwatch/settings.conf        # config (mount over it; entrypoint copies
#                                           example → settings.conf only if absent)
#   /var/lib/relwatch/                 # sqlite db (declare as a volume)
#
# `serve` is schedule-driven (cron) with SMTP digest email as the only egress,
# so the image exposes no HTTP port — mount a volume for /var/lib/relwatch
# and set a schedule via env/flag/settings.conf as needed (precedence:
# flag > env > settings.conf `schedule:` > default "0 * * * *").

FROM golang:1.26-alpine AS build
ARG RELWATCH_VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Same ldflags as Makefile (`-X main.version`, -trimpath, -buildvcs=false) but
# static (CGO_ENABLED=0) so the runtime stage stays tiny and fully static.
RUN CGO_ENABLED=0 go build \
    -trimpath -buildvcs=false \
    -ldflags "-s -w -X main.version=${RELWATCH_VERSION}" \
    -o /out/relwatch ./cmd/relwatch

FROM alpine:3.20
ARG RELWATCH_UID=10001
ARG RELWATCH_GID=10001
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g ${RELWATCH_GID} -S relwatch \
    && adduser -u ${RELWATCH_UID} -S -G relwatch -h /opt/relwatch relwatch

# Layout mirrors a systemd install: binary under /opt, config under /etc,
# state under /var/lib.
COPY --from=build /out/relwatch /opt/relwatch/relwatch
COPY settings.conf.example /etc/relwatch/settings.conf.example

# Entrypoint mirrors the Makefile install guard: only create settings.conf when
# it's absent, so a mounted config is never clobbered. Then exec serve. The
# database path is resolved from the config (system default: /var/lib/relwatch).
RUN printf '#!/bin/sh\nset -eu\nif [ ! -e /etc/relwatch/settings.conf ]; then\n  cp /etc/relwatch/settings.conf.example /etc/relwatch/settings.conf\nfi\nexec /opt/relwatch/relwatch serve --config /etc/relwatch/settings.conf\n' \
    > /opt/relwatch/entrypoint.sh \
    && chmod +x /opt/relwatch/entrypoint.sh \
    && mkdir -p /var/lib/relwatch \
    && chown -R relwatch:relwatch /opt/relwatch /etc/relwatch /var/lib/relwatch \
    && chmod 700 /var/lib/relwatch /etc/relwatch/settings.conf.example

VOLUME ["/var/lib/relwatch"]
USER relwatch
WORKDIR /opt/relwatch
# tzdata is installed; TZ is honored by Go's time packages at runtime.
ENTRYPOINT ["/opt/relwatch/entrypoint.sh"]
