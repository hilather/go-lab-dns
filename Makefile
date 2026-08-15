# LabDNS task runner. Tool versions are pinned; do not use @latest.

GO ?= go
export GOTOOLCHAIN ?= local
export GOPROXY ?= https://proxy.golang.org,direct

GOLANGCI_LINT_VERSION ?= v2.12.2
GOVULNCHECK_MOD ?= golang.org/x/vuln/cmd/govulncheck@v1.1.4
GOLANGCI_LINT_MOD ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: help format lint generate verify-generated test test-race test-fuzz-smoke \
	test-integration test-parity test-config-compat test-docs test-container \
	security-scan

help:
	@printf '%s\n' \
		'LabDNS Make targets (Go 1.26; module github.com/hilather/go-lab-dns)' \
		'  format              go fmt ./...' \
		'  lint                go vet + golangci-lint $(GOLANGCI_LINT_VERSION)' \
		'  generate            write testdata/generated/fixture.txt, api/capabilities/v1.json, api/openapi/v1.json, and api/mcp/v1.json' \
		'  generate            write testdata/generated/fixture.txt, api/capabilities/v1.json, api/openapi/v1.json, and api/metrics/v1alpha1.json' \
		'  verify-generated    fail if generate would change the fixture' \
		'  test                go test ./...' \
		'  test-race           go test -race ./...' \
		'  test-fuzz-smoke     execute the buildinfo and dnswire seed corpora' \
		'  test-docs           required documents and internal markdown links' \
		'  security-scan       govulncheck' \
		'  test-integration    unimplemented until later DNS/control-plane PRs' \
		'  test-parity         REST/MCP capability parity and MCP goldens' \
		'  test-config-compat  positive+negative v1alpha1 config fixtures' \
		'  test-container      build ghcr.io/hilather/labdns and check non-root/read-only/no-caps'

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
	@echo 'unimplemented until DNS/control-plane PRs: integration tests' >&2
	@exit 1

test-parity:
	$(GO) test ./internal/capabilities ./internal/control/rest ./internal/control/mcp -count=1

test-config-compat:
	$(GO) test ./internal/config -run TestConfigCompat -count=1

test-container:
	bash scripts/test-container.sh
