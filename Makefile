BINARY      := lazysql
TEA_BINARY  := lazytea
PREFIX      ?= $(HOME)/.local/bin

# Tagged version: the git tag if HEAD is one (e.g. v0.1.0), else <tag>-<n>-g<sha>,
# else the short sha. Create releases with: git tag vX.Y.Z
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DEV_VERSION := dev-$(COMMIT)

LDFLAGS     := -ldflags "-X main.version=$(VERSION)"
DEV_LDFLAGS := -ldflags "-X main.version=$(DEV_VERSION)"

.PHONY: build lazytea install install-dev install-lazytea install-lazysql \
	uninstall version test race vet tidy clean run help

## build: build lazytea (tagged version) to ./lazytea
build: lazytea

## lazytea: build the lazytea binary with the tagged version
lazytea:
	go build $(LDFLAGS) -o $(TEA_BINARY) ./cmd/lazytea

## install: build + install lazytea with the TAGGED version (git tag vX.Y.Z first)
install:
	go build $(LDFLAGS) -o $(TEA_BINARY) ./cmd/lazytea
	@mkdir -p $(PREFIX)
	@rm -f $(PREFIX)/$(TEA_BINARY)
	install -m 0755 $(TEA_BINARY) $(PREFIX)/$(TEA_BINARY)
	@echo "installed $(TEA_BINARY) -> $(PREFIX)/$(TEA_BINARY)  (version=$(VERSION))"

## install-dev: build + install lazytea with a DEV version marker (dev-<sha>)
install-dev:
	go build $(DEV_LDFLAGS) -o $(TEA_BINARY) ./cmd/lazytea
	@mkdir -p $(PREFIX)
	@rm -f $(PREFIX)/$(TEA_BINARY)
	install -m 0755 $(TEA_BINARY) $(PREFIX)/$(TEA_BINARY)
	@echo "installed $(TEA_BINARY) -> $(PREFIX)/$(TEA_BINARY)  (version=$(DEV_VERSION))"

## install-lazytea: alias for install
install-lazytea: install

## install-lazysql: build + install the legacy tview lazysql binary
install-lazysql:
	go build $(LDFLAGS) -o $(BINARY) .
	@mkdir -p $(PREFIX)
	@rm -f $(PREFIX)/$(BINARY)
	install -m 0755 $(BINARY) $(PREFIX)/$(BINARY)
	@echo "installed $(BINARY) -> $(PREFIX)/$(BINARY)  (version=$(VERSION))"

## uninstall: remove installed lazytea + lazysql binaries
uninstall:
	rm -f $(PREFIX)/$(TEA_BINARY) $(PREFIX)/$(BINARY)

## version: print the version strings install / install-dev would embed
version:
	@echo "tagged (make install):     $(VERSION)"
	@echo "dev    (make install-dev): $(DEV_VERSION)"

## test: run tests
test:
	go test ./...

## race: run tests with the race detector
race:
	go test -race ./...

## vet: run go vet
vet:
	go vet ./...

## tidy: go mod tidy
tidy:
	go mod tidy

## clean: remove built binaries
clean:
	rm -f $(BINARY) $(TEA_BINARY)

## run: build + run lazytea
run: lazytea
	./$(TEA_BINARY)

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -E 's/^## //'
