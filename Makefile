PREFIX  ?= $(HOME)/.local
UNITDIR ?= $(HOME)/.config/systemd/user
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test install install-units uninstall-units

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/lq ./cmd/lq

test:
	go vet ./...
	go test -race ./...

install: build
	install -Dm755 bin/lq $(PREFIX)/bin/lq

install-units:
	install -Dm644 -t $(UNITDIR) systemd/*
	systemctl --user daemon-reload
	systemctl --user enable --now lifequest-mt.timer lifequest-snapshot.path

uninstall-units:
	-systemctl --user disable --now lifequest-mt.timer lifequest-snapshot.path
	cd systemd && for u in *; do rm -f $(UNITDIR)/$$u; done
	systemctl --user daemon-reload
