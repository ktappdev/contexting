VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "0.0.1")

.PHONY: build install test

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/contexting .

install:
	go install -ldflags "-X main.version=$(VERSION)" .

test:
	go test ./...
