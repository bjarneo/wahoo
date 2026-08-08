PREFIX ?= $(HOME)/.local
BIN_DIR ?= $(PREFIX)/bin
VERSION ?= dev

.PHONY: build install test

build:
	mkdir -p bin
	go build -ldflags "-X main.version=$(VERSION)" -o bin/wahoo ./cmd/wahoo

install: build
	mkdir -p $(BIN_DIR)
	install -m 0755 bin/wahoo $(BIN_DIR)/wahoo

test:
	go test ./...
	go vet ./...
