.PHONY: build test lint install clean

build:
	go build -o bin/laccd-courses-pp-cli ./cmd/laccd-courses-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/laccd-courses-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/laccd-courses-pp-mcp ./cmd/laccd-courses-pp-mcp

install-mcp:
	go install ./cmd/laccd-courses-pp-mcp

build-all: build build-mcp
