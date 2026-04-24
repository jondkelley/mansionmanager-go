BINARY     := palace-manager
CMD        := ./cmd/palace-manager
INSTALL    := /usr/local/bin/$(BINARY)
SCRIPTS    := /usr/local/lib/palace-manager/scripts
# GitHub owner/repo for install-release (no https://github.com/); optional if origin is github.com
RELEASE_REPO ?=

.PHONY: build install push clean dist install-release help

help:
	@echo "palace-manager Makefile"
	@echo ""
	@echo "  make build              — compile $(BINARY) for this machine"
	@echo "  make install            — install binary + scripts on this host (needs root)"
	@echo "  make push HOST=user@h   — cross-build for remote Linux arch and deploy via deploy/push.sh"
	@echo "  make install-release VERSION=1.2.3 HOST=user@h [RELEASE_REPO=owner/repo]"
	@echo "                          — SSH to host, download GitHub release for remote arch, run install"
	@echo "  make dist TAG=v0.1.0 [ASSET_ARCH=amd64] — local release tarball under dist/ (uses go env GOOS/GOARCH)"
	@echo "  make clean"

build:
	go build -o $(BINARY) $(CMD)

install: build
	install -m 0755 $(BINARY) $(INSTALL)
	mkdir -p $(SCRIPTS)
	install -m 0755 scripts/provision-palace.sh $(SCRIPTS)/provision-palace.sh
	install -m 0755 scripts/update-pserver.sh $(SCRIPTS)/update-pserver.sh
	install -m 0755 scripts/gen-media-nginx.sh /usr/local/bin/gen-media-nginx.sh
	mkdir -p /etc/palace-manager

# Cross-compile and deploy to a remote host.
# Usage: make push HOST=root@1.2.3.4
push:
	@if [ -z "$(HOST)" ]; then echo "Usage: make push HOST=user@host" >&2; exit 1; fi
	./deploy/push.sh $(HOST)

# Build a release-style tarball locally (same layout as GitHub Releases).
# Usage: make dist TAG=v1.2.3
# Optional: ASSET_ARCH=armv7 when cross-compiling GOARCH=arm (filename label).
dist:
	@if [ -z "$(TAG)" ]; then echo "Usage: make dist TAG=v1.2.3 [ASSET_ARCH=...]" >&2; exit 1; fi
	TAG=$(TAG) GOOS=$${GOOS:-$$(go env GOOS)} GOARCH=$${GOARCH:-$$(go env GOARCH)} ASSET_ARCH=$${ASSET_ARCH:-$$(go env GOARCH)} ./scripts/package-release.sh

# Install a published release onto a remote Linux host (detects arch over SSH).
# Usage: make install-release VERSION=1.2.3 HOST=root@server RELEASE_REPO=myorg/palaceserver-js
install-release:
	@if [ -z "$(VERSION)" ] || [ -z "$(HOST)" ]; then \
	  echo "Usage: make install-release VERSION=1.2.3 HOST=user@host [RELEASE_REPO=owner/repo]" >&2; \
	  exit 1; \
	fi
	@RELEASE_REPO="$(RELEASE_REPO)" VERSION="$(VERSION)" HOST="$(HOST)" ./deploy/install-release.sh

clean:
	rm -f $(BINARY)
	rm -rf dist/
