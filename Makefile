# LabDNS task runner. Tool versions are pinned; do not use @latest.

GO ?= go
NODE ?= node
WEB_DIR := web
export GOTOOLCHAIN ?= local
export GOPROXY ?= https://proxy.golang.org,direct

GOLANGCI_LINT_VERSION ?= v2.12.2
GOVULNCHECK_MOD ?= golang.org/x/vuln/cmd/govulncheck@v1.1.4
GOLANGCI_LINT_MOD ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: help format lint generate verify-generated test test-race test-fuzz-smoke \
	test-integration test-parity test-config-compat test-docs test-container \
	security-scan test-changelog release-diff web-npm-ci web-test web-build web-generate

help:
	@printf '%s\n' \
		'LabDNS Make targets (Go 1.26; module github.com/hilather/go-lab-dns)' \
		'  format              go fmt ./...' \
		'  lint                go vet + golangci-lint $(GOLANGCI_LINT_VERSION)' \
		'  generate            write fixture + generated public surfaces (OpenAPI, MCP, capabilities, metrics, CLI help, errors)' \
		'  verify-generated    fail if generate would change any generated file' \
		'  test                go test ./...' \
		'  test-race           go test -race ./...' \
		'  test-fuzz-smoke     execute the buildinfo and dnswire seed corpora' \
		'  test-docs           required documents, metadata, links, and example YAML' \
		'  security-scan       govulncheck' \
		'  test-integration    short interop + soak/flood/admission (CI-safe)' \
		'  test-parity         REST/MCP capability parity and MCP goldens' \
		'  test-config-compat  positive+negative v1alpha1 config fixtures' \
		'  test-container      build ghcr.io/hilather/labdns and check non-root/read-only/no-caps' \
		'  test-changelog      fail if observable paths changed without CHANGELOG.md' \
		'  release-diff        compare public surfaces: make release-diff FROM=prev TO=HEAD NOTES=docs/releases/vX.Y.Z.md' \
		'  web-npm-ci          npm ci in web/ (shared by web-test and web-build)' \
		'  web-test            Vitest unit tests in web/ (fail closed if Node is missing; not Playwright)' \
		'  web-build           Vite production build to web/dist only (fail closed if Node is missing)' \
		'  web-generate        OpenAPI TypeScript client (unimplemented until UI-002)'

format:
	$(GO) fmt ./...

lint:
	$(GO) vet ./...
	$(GO) run $(GOLANGCI_LINT_MOD) run ./...

generate:
	$(GO) run ./scripts/generate

verify-generated:
	$(GO) run ./scripts/generate -check

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-fuzz-smoke:
	$(GO) test ./internal/buildinfo -fuzz=FuzzInfoString -fuzztime=5s -count=1
	$(GO) test ./internal/dnswire -fuzz=FuzzParse -fuzztime=10s -count=1

test-docs:
	$(GO) run ./scripts/checkdocs

security-scan:
	$(GO) run $(GOVULNCHECK_MOD) ./...

test-integration:
	$(GO) test ./internal/interop ./internal/perf ./benches -count=1 -timeout=90s

test-parity:
	$(GO) test ./internal/capabilities ./internal/control/rest ./internal/control/mcp -count=1

test-config-compat:
	$(GO) test ./internal/config -run TestConfigCompat -count=1

test-container:
	bash scripts/test-container.sh

test-changelog:
	$(GO) run ./scripts/checkchangelog

web-npm-ci:
	@command -v $(NODE) >/dev/null || { echo 'web-npm-ci: node 22.14.0 is required' >&2; exit 1; }
	cd $(WEB_DIR) && npm ci

web-generate: web-npm-ci
	@command -v $(NODE) >/dev/null || { echo 'web-generate: node 22.14.0 is required' >&2; exit 1; }
	cd $(WEB_DIR) && npm run generate

web-test: web-npm-ci
	@command -v $(NODE) >/dev/null || { echo 'web-test: node 22.14.0 is required' >&2; exit 1; }
	cd $(WEB_DIR) && npm test

web-build: web-npm-ci
	@command -v $(NODE) >/dev/null || { echo 'web-build: node 22.14.0 is required' >&2; exit 1; }
	cd $(WEB_DIR) && npm run build
	test -f web/dist/index.html

# FROM and TO are git refs. NOTES is optional and required for a tag gate.
FROM ?=
TO ?= HEAD
NOTES ?=
release-diff:
	@test -n "$(FROM)" || (echo 'release-diff: FROM is required (previous tag or empty-tree SHA)' >&2; exit 1)
	$(GO) run ./scripts/release-diff $(if $(NOTES),-notes $(NOTES),) $(FROM) $(TO)
