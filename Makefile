VERSION ?= dev

.PHONY: build test lint install clean cross-compile

build:
	go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o bin/xreview ./cmd/xreview

test:
	go test ./internal/... ./cmd/...

lint:
	golangci-lint run ./...

install:
	go install -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" ./cmd/xreview

clean:
	rm -rf bin/ dist/

cross-compile:
	@mkdir -p dist/
	GOOS=linux  GOARCH=amd64 go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o dist/xreview-linux-amd64  ./cmd/xreview
	GOOS=linux  GOARCH=arm64 go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o dist/xreview-linux-arm64  ./cmd/xreview
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o dist/xreview-darwin-amd64 ./cmd/xreview
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o dist/xreview-darwin-arm64 ./cmd/xreview
