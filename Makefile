PREFIX  ?= $(HOME)/.local
UNITDIR ?= $(HOME)/.config/systemd/user
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test check-widgets install install-units uninstall-units install-board

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/acta ./cmd/acta

test:
	go vet ./...
	go test -race ./...

# Renders every widget with the vendored OWL runtime under node.
check-widgets: build
	board/test/check.sh

install: build
	install -Dm755 bin/acta $(PREFIX)/bin/acta

install-units:
	install -Dm644 -t $(UNITDIR) systemd/*
	systemctl --user daemon-reload
	systemctl --user enable --now acta-mt.timer acta-inbox.timer acta-snapshot.path

uninstall-units:
	-systemctl --user disable --now acta-mt.timer acta-inbox.timer acta-snapshot.path
	cd systemd && for u in *; do rm -f $(UNITDIR)/$$u; done
	systemctl --user daemon-reload

install-board:
	board/install.sh
