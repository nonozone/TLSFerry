.PHONY: build fmt test verify

build:
	go build -o bin/tlsferry ./cmd/tlsferry

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

verify: fmt test build
