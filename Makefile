VERSION ?= dev

.PHONY: build test lint install clean

build:
	go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o bin/xreview ./cmd/xreview

test:
	go test ./internal/... ./cmd/...

lint:
	golangci-lint run ./...

install:
	go install -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" ./cmd/xreview

clean:
	rm -rf bin/
