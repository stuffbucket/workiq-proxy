BINARY = workiq-proxy
MODULE = ./cmd/workiq-proxy

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w

PLATFORMS = \
	darwin/amd64/workiq-proxy-darwin-x64 \
	darwin/arm64/workiq-proxy-darwin-arm64 \
	linux/amd64/workiq-proxy-linux-x64 \
	linux/arm64/workiq-proxy-linux-arm64 \
	windows/amd64/workiq-proxy-win-x64.exe \
	windows/arm64/workiq-proxy-win-arm64.exe

.PHONY: build test vet clean all

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(MODULE)

test:
	go test -v -count=1 $(MODULE)

vet:
	go vet $(MODULE)

clean:
	rm -f $(BINARY) workiq-proxy-*

all: clean vet test
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1) \
		GOARCH=$$(echo $$platform | cut -d/ -f2) \
		OUTPUT=$$(echo $$platform | cut -d/ -f3); \
		echo "Building $$OUTPUT ($$GOOS/$$GOARCH)"; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags "$(LDFLAGS)" -o $$OUTPUT $(MODULE); \
	done
