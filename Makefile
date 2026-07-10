VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "0.0.1")

.PHONY: build install test

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/ctxt .

install:
	go build -ldflags "-X main.version=$(VERSION)" -o $(shell go env GOPATH)/bin/ctxt .

test:
	go test ./...
