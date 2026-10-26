#!/usr/bin/env bash
#
# install-relwatch.sh — install relwatch as a systemd service.
#
# Downloads the latest release binary from GitHub Releases, creates the
# unprivileged `relwatch` system user, drops a hardened systemd unit, and
# enables a scheduled poll that emails a digest of new releases.
#
# Quick start:
#   curl -fsSL https://raw.githubusercontent.com/epiclabs-cc/relwatch/main/install-relwatch.sh | sudo bash
#
# Or, from a checkout:  sudo ./install-relwatch.sh [--dl-url URL]
# Uninstall:            sudo ./install-relwatch.sh --uninstall
#
set -euo pipefail

SERVICE_USER="relwatch"
DEFAULT_PREFIX="/opt/relwatch"
CONFIG_DIR="/etc/relwatch"
CONFIG_FILE="${CONFIG_DIR}/settings.conf"
DATA_DIR="/var/lib/relwatch"
RELEASES_URL="https://github.com/epiclabs-cc/relwatch/releases"
DEFAULT_SCHEDULE="0 * * * *"
UNIT_PATH="/etc/systemd/system/relwatch.service"

PREFIX="$DEFAULT_PREFIX"
DL_URL="$RELEASES_URL"
TAG="latest"
UNINSTALL=0
NO_ENABLE=0
NON_INTERACTIVE=0
SCHEDULE=""

SMTP_HOST="" SMTP_PORT="" SMTP_USER="" SMTP_PASS="" SMTP_FROM="" SMTP_TO="" SMTP_TLS=""

log()  { printf '[i] %s\n' "$*"; }
warn() { printf '[!] %s\n' "$*" >&2; }
die()  { printf '[x] %s\n' "$*" >&2; exit 1; }

usage() {
	cat <<EOF
Usage: install-relwatch.sh [options]

Installs relwatch as a systemd service. Must be run as root.

Options:
  --prefix DIR        install prefix for the binary (default: ${DEFAULT_PREFIX})
  --tag TAG           release tag to install (default: latest)
  --dl-url URL        GitHub Releases base URL (default: ${RELEASES_URL})
  --schedule CRON     5-field cron schedule (default: "${DEFAULT_SCHEDULE}")
  --smtp-host HOST    SMTP server (enables the digest)
  --smtp-port PORT    SMTP port (default: 587; 465 implies implicit TLS)
  --smtp-user USER    SMTP AUTH username
  --smtp-pass PASS    SMTP AUTH password
  --smtp-from FROM    From: address (default: the SMTP username)
  --smtp-to LIST      comma/space-separated digest recipients
  --smtp-tls MODE     auto (default), starttls, implicit, none
  --no-enable         install the unit but do not start/enable it
  --non-interactive   never prompt; use flags/defaults only
  --uninstall         stop + remove the service (keeps ${CONFIG_FILE} and ${DATA_DIR})
  -h, --help          show this help

With no SMTP flags on a terminal, the installer prompts for the SMTP settings.
EOF
}

parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
			--prefix)         PREFIX="${2:?}"; shift 2 ;;
			--tag)            TAG="${2:?}"; shift 2 ;;
			--dl-url)         DL_URL="${2:?}"; shift 2 ;;
			--schedule)       SCHEDULE="${2:?}"; shift 2 ;;
			--smtp-host)      SMTP_HOST="${2:?}"; shift 2 ;;
			--smtp-port)      SMTP_PORT="${2:?}"; shift 2 ;;
			--smtp-user)      SMTP_USER="${2:?}"; shift 2 ;;
			--smtp-pass)      SMTP_PASS="${2:?}"; shift 2 ;;
			--smtp-from)      SMTP_FROM="${2:?}"; shift 2 ;;
			--smtp-to)        SMTP_TO="${2:?}"; shift 2 ;;
			--smtp-tls)       SMTP_TLS="${2:?}"; shift 2 ;;
			--no-enable)      NO_ENABLE=1; shift ;;
			--non-interactive) NON_INTERACTIVE=1; shift ;;
			--uninstall)      UNINSTALL=1; shift ;;
			-h|--help)        usage; exit 0 ;;
			*) die "unknown option: $1 (try --help)" ;;
		esac
	done
}

require_root() {
	[ "$(id -u)" -eq 0 ] && return 0
	if [ -f "$0" ] && command -v sudo >/dev/null 2>&1; then
		log "not root; re-running with sudo"
		exec sudo -E "$0" "$@"
	fi
	die "must run as root (try: curl -fsSL https://raw.githubusercontent.com/epiclabs-cc/relwatch/main/install-relwatch.sh | sudo bash)"
}

detect_arch() {
	case "$(uname -m)" in
		x86_64|amd64)  ARCH="amd64" ;;
		aarch64|arm64) ARCH="arm64" ;;
		*) die "unsupported architecture: $(uname -m) (only amd64/arm64 are released)" ;;
	esac
}

fetch() {
	local url="$1" out="$2"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 3 "$url" -o "$out"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$out" --tries=3 "$url"
	else
		die "need curl or wget to download the release"
	fi
}

verify_binary() {
	local magic
	magic="$(head -c 4 "$1" | od -An -tx1 | tr -d ' \n')"
	if [ "$magic" != "7f454c46" ]; then
		die "downloaded file is not a Linux ELF binary (magic=${magic:-empty});
    the mirror likely returned an HTML or error page — refusing to install"
	fi
}

verify_checksum() {
	local file="$1" base="$2" name="$3" sums expected actual
	sums="$(mktemp)"
	if ! fetch "${base}/SHA256SUMS" "$sums" 2>/dev/null || [ ! -s "$sums" ]; then
		rm -f "$sums"
		warn "no SHA256SUMS at ${base}; skipping checksum verification"
		return 0
	fi
	expected="$(awk -v n="$name" '$NF == n { print $1 }' "$sums" | head -1)"
	rm -f "$sums"
	if [ -z "$expected" ]; then
		warn "no checksum entry for ${name}; skipping verification"
		return 0
	fi
	actual="$(sha256sum "$file" | awk '{ print $1 }')"
	if [ "$expected" != "$actual" ]; then
		die "checksum mismatch for ${name}:
    expected ${expected}
    got      ${actual}"
	fi
	log "checksum verified for ${name}"
}

download_binary() {
	local base url out
	if [ "$TAG" = "latest" ]; then
		base="${DL_URL}/latest/download"
	else
		base="${DL_URL}/download/${TAG}"
	fi
	url="${base}/relwatch-linux-${ARCH}"
	out="$(mktemp)"
	log "downloading relwatch-linux-${ARCH} (${TAG}) from ${DL_URL}"
	if ! fetch "$url" "$out"; then
		rm -f "$out"
		die "download failed: ${url}
    - is the release published? try --tag to pick a specific release"
	fi
	verify_binary "$out"
	verify_checksum "$out" "$base" "relwatch-linux-${ARCH}"
	install -m 0755 "$out" "${PREFIX}/relwatch"
	rm -f "$out"
}

create_user() {
	if ! getent group "$SERVICE_USER" >/dev/null 2>&1; then
		groupadd --system "$SERVICE_USER"
		log "created group ${SERVICE_USER}"
	fi
	if ! getent passwd "$SERVICE_USER" >/dev/null 2>&1; then
		useradd --system --gid "$SERVICE_USER" --home-dir "$PREFIX" \
			--shell /usr/sbin/nologin "$SERVICE_USER"
		log "created system user ${SERVICE_USER}"
	fi
}

prompt_smtp() {
	[ "$NON_INTERACTIVE" -eq 1 ] && return 0
	[ -t 0 ] || return 0
	[ -f "${CONFIG_FILE}" ] && return 0

	echo
	log "Email digest setup (press Enter to accept a default or skip)"
	[ -z "$SMTP_HOST" ] && read -rp "[?] SMTP host (blank to skip email): " SMTP_HOST || true
	if [ -n "$SMTP_HOST" ]; then
		[ -z "$SMTP_PORT" ] && { read -rp "[?] SMTP port [587]: " SMTP_PORT || true; SMTP_PORT="${SMTP_PORT:-587}"; }
		[ -z "$SMTP_USER" ] && { read -rp "[?] SMTP username (blank for none): " SMTP_USER || true; }
		if [ -n "$SMTP_USER" ] && [ -z "$SMTP_PASS" ]; then
			read -rsp "[?] SMTP password: " SMTP_PASS || true; echo
		fi
		[ -z "$SMTP_FROM" ] && { read -rp "[?] From address [${SMTP_USER:-relwatch@example.com}]: " SMTP_FROM || true; }
		[ -z "$SMTP_TO" ] && read -rp "[?] Digest recipient(s), comma-separated: " SMTP_TO || true
	fi
	[ -z "$SCHEDULE" ] && { read -rp "[?] Poll schedule [${DEFAULT_SCHEDULE}]: " SCHEDULE || true; }
	echo
}

yaml_escape() {
	printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

write_settings() {
	local f="${CONFIG_FILE}"
	if [ -f "$f" ]; then
		log "keeping existing ${f}"
		return 0
	fi
	install -d -m 0755 "$CONFIG_DIR"
	local enabled="false"
	[ -n "$SMTP_HOST" ] && enabled="true"
	SMTP_TO="${SMTP_TO//,/ }"

	{
		echo "# relwatch settings — generated by install-relwatch.sh"
		echo "# See docs/configuration.md; keep this file out of version control."
		echo "db: \"${DATA_DIR}/relwatch.db\""
		echo "schedule: \"$(yaml_escape "${SCHEDULE:-$DEFAULT_SCHEDULE}")\""
		echo
		echo "smtp:"
		echo "  enabled: ${enabled}"
		echo "  host: \"$(yaml_escape "${SMTP_HOST:-smtp.example.com}")\""
		echo "  port: ${SMTP_PORT:-587}"
		echo "  tls: \"$(yaml_escape "${SMTP_TLS:-auto}")\""
		[ -n "$SMTP_USER" ] && echo "  username: \"$(yaml_escape "$SMTP_USER")\""
		[ -n "$SMTP_PASS" ] && echo "  password: \"$(yaml_escape "$SMTP_PASS")\""
		echo "  from: \"$(yaml_escape "${SMTP_FROM:-${SMTP_USER:-relwatch@example.com}}")\""
		echo "  to:"
		if [ -n "$SMTP_TO" ]; then
			for r in $SMTP_TO; do echo "    - \"$(yaml_escape "$r")\""; done
		else
			echo "    - \"you@example.com\""
		fi
	} > "$f"
	chmod 0600 "$f"
	chown "$SERVICE_USER:$SERVICE_USER" "$f" 2>/dev/null || true
	log "wrote ${f}"
}

write_unit() {
	log "writing ${UNIT_PATH}"
	cat > "$UNIT_PATH" <<EOF
[Unit]
Description=relwatch — poll providers and email release digests
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${PREFIX}
ExecStart=${PREFIX}/relwatch serve \\
  --config ${CONFIG_FILE}
Restart=on-failure
RestartSec=10s
NoNewPrivileges=yes
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths=${DATA_DIR}
ProtectKernelTunables=yes
ProtectKernelModules=yes
PrivateDevices=yes
CapabilityBoundingSet=
SystemCallFilter=@system-service

[Install]
WantedBy=multi-user.target
EOF
	chmod 0644 "$UNIT_PATH"
}

enable_service() {
	systemctl daemon-reload
	if [ "$NO_ENABLE" -eq 1 ]; then
		log "unit installed but not started (--no-enable); start it with: systemctl enable --now relwatch"
		return 0
	fi
	systemctl enable --now relwatch
	log "relwatch enabled and started"
}

do_install() {
	detect_arch
	log "installing to ${PREFIX}"

	install -d "$PREFIX"
	install -d -m 0755 "$CONFIG_DIR"
	install -d -m 0700 "$DATA_DIR"

	download_binary

	create_user
	chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"

	prompt_smtp
	write_settings
	write_unit
	enable_service

	echo
	log "done. Useful commands:"
	echo "    systemctl status relwatch"
	echo "    journalctl -u relwatch -f"
	echo "    \$EDITOR ${CONFIG_FILE}   # then: systemctl restart relwatch"
	echo "    sudo -u ${SERVICE_USER} ${PREFIX}/relwatch project add <name> <owner/repo>"
	echo "        # add tracked projects to the service database (${DATA_DIR}/relwatch.db)"
}

do_uninstall() {
	if systemctl list-unit-files 2>/dev/null | grep -q '^relwatch\.service'; then
		systemctl disable --now relwatch 2>/dev/null || true
	fi
	rm -f "$UNIT_PATH"
	rm -f "${PREFIX}/relwatch"
	systemctl daemon-reload 2>/dev/null || true
	log "removed binary + unit; kept ${CONFIG_FILE} and ${DATA_DIR}"
	log "the '${SERVICE_USER}' system user is left in place (remove with: userdel ${SERVICE_USER})"
}

main() {
	parse_args "$@"
	require_root "$@"
	if [ "$UNINSTALL" -eq 1 ]; then
		do_uninstall
	else
		do_install
	fi
}

main "$@"
