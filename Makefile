BINARY     := palace-manager
CMD        := ./cmd/palace-manager
INSTALL    := /usr/local/bin/$(BINARY)
SCRIPTS    := /usr/local/lib/palace-manager/scripts

.PHONY: build install push clean

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

clean:
	rm -f $(BINARY)
