BINARY     := lazysql
TEA_BINARY := lazytea
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
PREFIX  ?= $(HOME)/.local/bin

.PHONY: build install lazytea install-lazytea test race vet tidy clean run

build:
	go build $(LDFLAGS) -o $(BINARY) .

# symlink the built binary onto PATH (matches the ~/.local/bin setup)
install: build
	mkdir -p $(PREFIX)
	ln -sf $(CURDIR)/$(BINARY) $(PREFIX)/$(BINARY)

lazytea:
	go build $(LDFLAGS) -o $(TEA_BINARY) ./cmd/lazytea

install-lazytea: lazytea
	mkdir -p $(PREFIX)
	ln -sf $(CURDIR)/$(TEA_BINARY) $(PREFIX)/$(TEA_BINARY)

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY) $(TEA_BINARY)

run: build
	./$(BINARY)
