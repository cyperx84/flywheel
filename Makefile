.PHONY: build install test clean vet lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
PREFIX ?= /usr/local

build:
	go build -ldflags "-s -w -X github.com/cyperx84/flywheel/cmd/flywheel/cmd.version=$(VERSION)" -o flywheel ./cmd/flywheel/

install: build
	install -m755 flywheel $(PREFIX)/bin/flywheel

test:
	go test ./... -v

vet:
	go vet ./...

clean:
	rm -f flywheel

release:
	goreleaser release --clean
