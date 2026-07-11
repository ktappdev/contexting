VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "0.0.1")

.PHONY: build install test

build:
	go build -ldflags "-X github.com/ktappdev/contexting.Version=$(VERSION)" -o bin/ctxt ./cmd/ctxt

install:
	go build -ldflags "-X github.com/ktappdev/contexting.Version=$(VERSION)" -o $(shell go env GOPATH)/bin/ctxt ./cmd/ctxt

test:
	go test ./...
