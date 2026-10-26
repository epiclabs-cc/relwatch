GO ?= go

BINARY  := relwatch
TAG     := $(shell git describe --tags --exact-match 2>/dev/null)
VERSION ?= $(if $(TAG),$(TAG:v%=%),dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS := -trimpath -buildvcs=false

PREFIX  ?= /opt/relwatch
DESTDIR ?=
CONFIG_DIR ?= /etc/relwatch
CONFIG_FILE = $(CONFIG_DIR)/settings.conf
DBDIR   ?= /var/lib/relwatch
SERVICE_USER := relwatch
UNIT     = contrib/relwatch.service
UNITDIR  = $(DESTDIR)/etc/systemd/system
BIN     := bin/$(BINARY)

.PHONY: build release test vet clean version install uninstall

version:
	@echo $(VERSION)

build:
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/relwatch

release:
	mkdir -p bin/release
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/release/$(BINARY)-linux-amd64 ./cmd/relwatch
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/release/$(BINARY)-linux-arm64 ./cmd/relwatch
	printf '%s\n' "$(VERSION)" > bin/release/VERSION

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

clean:
	rm -rf bin

# install — drop the binary under PREFIX, config under /etc, state under /var/lib.
#
#	PREFIX/relwatch       binary
#	/etc/relwatch/settings.conf   config (copied from settings.conf.example only if
#	                          absent; never clobbers an existing file holding the SMTP
#	                          password). Records db: $(DBDIR)/relwatch.db
#	/var/lib/relwatch/    SQLite database (created, owned by the service user)
#	/etc/systemd/system/     relwatch.service
#
# On a live system (DESTDIR empty) this also creates the unprivileged
# `relwatch` system user the unit runs as, then reloads + enables the unit.
# Run via `sudo make install`.
install: build
	install -d $(DESTDIR)$(PREFIX)
	install -m 0755 $(BIN) $(DESTDIR)$(PREFIX)/$(BINARY)
	install -d $(DESTDIR)$(CONFIG_DIR)
	install -d -m 0700 $(DESTDIR)$(DBDIR)
	@if [ ! -f $(DESTDIR)$(CONFIG_FILE) ]; then \
		install -m 0600 settings.conf.example $(DESTDIR)$(CONFIG_FILE); \
		{ echo; echo "# Database path recorded by 'make install'."; echo "db: \"$(DBDIR)/relwatch.db\""; } >> $(DESTDIR)$(CONFIG_FILE); \
		echo "relwatch: wrote $(CONFIG_FILE) (edit SMTP/password as needed)"; \
	else \
		echo "relwatch: $(CONFIG_FILE) exists, not overwritten"; \
	fi
	install -d $(UNITDIR)
	sed -e 's|__PREFIX__|$(PREFIX)|g' -e 's|__CONFIG_FILE__|$(CONFIG_FILE)|g' -e 's|__DATA_DIR__|$(DBDIR)|g' $(UNIT) > $(UNITDIR)/relwatch.service
	@if [ -z "$(DESTDIR)" ]; then \
		getent group $(SERVICE_USER) >/dev/null || groupadd --system $(SERVICE_USER); \
		getent passwd $(SERVICE_USER) >/dev/null || useradd --system --gid $(SERVICE_USER) --home-dir $(PREFIX) --shell /usr/sbin/nologin $(SERVICE_USER); \
		chown -R $(SERVICE_USER):$(SERVICE_USER) $(DBDIR); \
		chown $(SERVICE_USER):$(SERVICE_USER) $(CONFIG_FILE); \
		systemctl daemon-reload; \
		systemctl enable --now relwatch; \
	else \
		echo "relwatch: DESTDIR set — staged install; skipping user creation and systemctl"; \
	fi
	@echo "relwatch: installed to $(PREFIX), unit ready at $(UNITDIR)/relwatch.service"

uninstall:
	@if [ -z "$(DESTDIR)" ]; then systemctl disable --now relwatch 2>/dev/null || true; fi
	-rm -f $(DESTDIR)$(PREFIX)/$(BINARY)
	-rm -f $(UNITDIR)/relwatch.service
	@if [ -z "$(DESTDIR)" ]; then systemctl daemon-reload 2>/dev/null || true; fi
	@echo "relwatch: removed binary + unit (kept $(CONFIG_FILE) and $(DBDIR) data)"
