BINARY  := lazysql
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
PREFIX  ?= $(HOME)/.local/bin

.PHONY: build install test race vet tidy clean run

build:
	go build $(LDFLAGS) -o $(BINARY) .

# symlink the built binary onto PATH (matches the ~/.local/bin setup)
install: build
	mkdir -p $(PREFIX)
	ln -sf $(CURDIR)/$(BINARY) $(PREFIX)/$(BINARY)

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)

run: build
	./$(BINARY)
