.PHONY: help build install uninstall fmt fmt-check test vet security validate-example verify release-snapshot clean

BINARY ?= tlsferry
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
LDFLAGS := -s -w -X github.com/nonozone/TLSFerry/internal/cli.version=$(VERSION)
GOVULNCHECK_VERSION ?= v1.6.0

help:
	@echo "TLSFerry CE development and release commands"
	@echo "  make build             Build bin/$(BINARY) with version metadata"
	@echo "  make install           Install into $(BINDIR)"
	@echo "  make uninstall         Remove $(BINDIR)/$(BINARY)"
	@echo "  make test              Run the Go test suite"
	@echo "  make security          Scan reachable Go code for known vulnerabilities"
	@echo "  make verify            Run formatting, tests, vet, security, build, and config checks"
	@echo "  make release-snapshot  Build local release archives with GoReleaser"

build:
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/tlsferry

install: build
	install -d "$(BINDIR)"
	install -m 0755 "bin/$(BINARY)" "$(BINDIR)/$(BINARY)"

uninstall:
	rm -f "$(BINDIR)/$(BINARY)"

fmt:
	gofmt -w ./cmd ./internal

fmt-check:
	@files="$$(gofmt -l ./cmd ./internal)"; if [ -n "$$files" ]; then echo "Go files need formatting:"; echo "$$files"; exit 1; fi

test:
	go test ./... -count=1

vet:
	go vet ./...

security:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

validate-example:
	go run ./cmd/tlsferry validate --config config.example.json

verify: fmt-check test vet security build validate-example

release-snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist
