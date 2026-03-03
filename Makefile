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

.PHONY: build test vet clean all setup lint lint-go lint-js audit audit-go audit-js release docs licenses

# ── Development setup ────────────────────────────────────────────

setup: setup-go setup-js setup-hooks

setup-go:
	@echo "==> Installing golangci-lint..."
	@command -v golangci-lint >/dev/null 2>&1 || \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@echo "==> Go toolchain ready"

setup-js:
	@echo "==> Installing ESLint and JS dev dependencies..."
	npm install --no-audit --no-fund
	@echo "==> JS toolchain ready"

setup-hooks:
	@echo "==> Installing git pre-commit hook..."
	@mkdir -p .githooks
	@printf '#!/bin/sh\nmake pre-commit\n' > .githooks/pre-commit
	@chmod +x .githooks/pre-commit
	@git config core.hooksPath .githooks
	@echo "==> Pre-commit hook installed"

# ── Linting ──────────────────────────────────────────────────────

lint: lint-go lint-js

lint-go:
	golangci-lint run $(MODULE)

lint-js:
	npx eslint npm/ contrib/tools/

# ── Build / Test ─────────────────────────────────────────────────

build: docs
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(MODULE)

test:
	go test -race -count=1 $(MODULE)

vet:
	go vet $(MODULE)

# ── Doc Generation ───────────────────────────────────────────────

docs:
	go test -run TestGenerateDocs $(MODULE) -args -update-docs

# ── License Collection ───────────────────────────────────────────

licenses:
	@echo "==> Collecting third-party licenses..."
	go run cmd/workiq-proxy/gen_licenses.go
	@cp LICENSE cmd/workiq-proxy/LICENSE
	@cp THIRD_PARTY_DISCLOSURES.md cmd/workiq-proxy/THIRD_PARTY_DISCLOSURES.md
	@echo "==> Copied LICENSE and THIRD_PARTY_DISCLOSURES.md into cmd/workiq-proxy/ for embedding"
	@cp LICENSE npm/LICENSE
	@cp THIRD_PARTY_DISCLOSURES.md npm/THIRD_PARTY_DISCLOSURES.md
	@echo "==> Copied LICENSE and THIRD_PARTY_DISCLOSURES.md into npm/ for distribution"

# ── Vulnerability / Audit ────────────────────────────────────────

audit: audit-go audit-js

audit-go:
	@echo "==> Scanning Go dependencies for known vulnerabilities..."
	go run golang.org/x/vuln/cmd/govulncheck@latest $(MODULE)/

audit-js:
	@echo "==> Auditing npm dependencies..."
	cd npm && npm audit --omit=dev
	@echo "==> npm audit clean"

# ── Pre-commit ───────────────────────────────────────────────────

pre-commit: vet lint test build audit
	@echo "==> Pre-commit checks passed"

clean:
	rm -f $(BINARY) workiq-proxy-*
	rm -rf docs/dist docs/node_modules/.astro docs/.astro
	go clean -cache -testcache

all: clean lint test
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1) \
		GOARCH=$$(echo $$platform | cut -d/ -f2) \
		OUTPUT=$$(echo $$platform | cut -d/ -f3); \
		echo "Building $$OUTPUT ($$GOOS/$$GOARCH)"; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags "$(LDFLAGS)" -o $$OUTPUT $(MODULE); \
	done

# ── Release ──────────────────────────────────────────────────────
# Usage: make release v=0.2.0

release:
ifndef v
	$(error Usage: make release v=0.2.0)
endif
	@echo "==> Running full build pipeline..."
	$(MAKE) all
	@echo "==> Tagging v$(v)..."
	git tag v$(v)
	@echo "==> Pushing tag v$(v) to origin..."
	git push origin v$(v)
	@echo "==> Release v$(v) triggered. GitHub Actions will create the release and publish to npm."
