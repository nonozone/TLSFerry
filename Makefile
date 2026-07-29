.PHONY: help build install uninstall fmt fmt-check test functional-smoke vet security validate-example verify release-audit release-snapshot artifact-smoke release-check clean

BINARY ?= tlsferry
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
LDFLAGS := -s -w -X github.com/nonozone/TLSFerry/internal/cli.version=$(VERSION)
GOVULNCHECK_VERSION ?= v1.6.0
GORELEASER_VERSION ?= v2.13.3
AUDIT_VERSION ?= v0.0.0-local
AUDIT_REVIEWER ?= local

help:
	@echo "TLSFerry CE development and release commands"
	@echo "  make build             Build bin/$(BINARY) with version metadata"
	@echo "  make install           Install into $(BINDIR)"
	@echo "  make uninstall         Remove $(BINDIR)/$(BINARY)"
	@echo "  make test              Run the Go test suite"
	@echo "  make functional-smoke  Exercise the credential-free CE CLI release flow"
	@echo "  make security          Scan reachable Go code for known vulnerabilities"
	@echo "  make verify            Run formatting, tests, vet, security, build, and config checks"
	@echo "  make release-audit     Audit candidate metadata and CE source boundaries"
	@echo "  make release-snapshot  Build local release archives with GoReleaser"
	@echo "  make artifact-smoke    Verify checksums, archive contents, and native version"
	@echo "  make release-check     Run the clean-worktree RC release gate"

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

functional-smoke:
	go test -tags release_smoke ./internal/cli -run '^TestReleaseFunctionalSmoke$$' -count=1 -v

vet:
	go vet ./...

security:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

validate-example:
	go run ./cmd/tlsferry validate --config config.example.json
	go run ./cmd/tlsferry validate --config config.release-smoke.example.json

verify: fmt-check test functional-smoke vet security build validate-example

release-audit:
	@go run ./internal/releaseaudit --version "$(AUDIT_VERSION)" --reviewer "$(AUDIT_REVIEWER)"

release-snapshot:
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) release --snapshot --clean

artifact-smoke:
	go run ./internal/releaseartifact --repository . --dist dist

release-check:
	@test -z "$$(git status --porcelain --untracked-files=no)" || (echo "release check requires a clean tracked worktree"; exit 1)
	$(MAKE) release-audit
	$(MAKE) verify
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) check
	$(MAKE) release-snapshot
	$(MAKE) artifact-smoke
	@test -z "$$(git status --porcelain --untracked-files=no)" || (echo "release check changed tracked files"; exit 1)

clean:
	rm -rf bin dist
