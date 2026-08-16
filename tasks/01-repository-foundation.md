# FND-001: Repository Foundation

Status: done
Recommended owner: Go/platform agent
Dependencies: None
Exclusive ownership: root build files, initial module layout, CI skeleton

## Goal

Create a minimal, secure, testable Go repository that encodes the architectural boundaries before feature implementation.

## Work items

- [x] Initialize the Go module with the agreed module path.
- [x] Create package directories from `docs/01-architecture.md` with package comments and no cyclic dependencies.
- [x] Add `cmd/labdns` with a placeholder version command and clean context-based shutdown wiring.
- [x] Add `Makefile` targets required by `AGENTS.md`, initially wired to real or explicit placeholder checks that fail when unimplemented rather than silently succeeding.
- [x] Add formatting, lint, unit, race, fuzz-smoke, generated-file, documentation, security-scan, and container-test CI jobs.
- [x] Add dependency update and vulnerability scan configuration.
- [x] Add license, code owners, pull-request template, issue templates, changelog, and release-note template.
- [x] Add build metadata package containing version, commit, build time policy, and supported protocol placeholders.
- [x] Add deterministic test helpers, fake clock interfaces, and test cleanup conventions.
- [x] Add a script or test that verifies required root documents exist.

## Required tests

- [x] A clean checkout passes format and unit checks.
- [x] Generated-file verification fails on an intentionally changed fixture and passes when regenerated.
- [x] Documentation-link checking runs in CI.
- [x] Race target is exercised by at least one concurrency test.
- [x] Fuzz-smoke target executes a seed corpus.
- [x] CI configuration itself has validation or a smoke path.
- [x] Regression test ensures no required Make target is a no-op success.

## Documentation updates

- [x] Replace placeholder module paths in examples.
- [x] Record chosen license.
- [x] Document local tool prerequisites and versions.
- [x] Update `README.md` with build and test commands.
- [x] Add an unreleased changelog entry for repository initialization.

## Acceptance criteria

- The repository builds on supported platforms.
- All required CI jobs are marked required in repository policy documentation.
- No feature code is coupled to a specific DNS or MCP library yet.
- `go test ./...` and the initial required checks pass.
- A failing CI demonstration has documented artifact retention and no bypass path.

## Handoff

- **Toolchain:** Go 1.26 (`go1.26.x`). Module `github.com/hilather/go-lab-dns`. golangci-lint `v2.12.2`. govulncheck `golang.org/x/vuln/cmd/govulncheck@v1.1.4`. License Apache-2.0. Image (later) `ghcr.io/hilather/labdns`.
- **Generated-file strategy:** `scripts/generate` writes `testdata/generated/fixture.txt` from `testdata/generated/source.txt` and `go.mod`. `make generate` writes; `make verify-generated` is `-check`. Later OpenAPI/JSON Schema/MCP/capability artifacts must plug into the same generate/verify pair.
- **Package dependency constraints:** see `docs/implementation-design.md` allowed import direction. `internal/model` imports nothing from wire, MCP, HTTP, snapshot, or telemetry. Third-party DNS/MCP types must not escape adapters.
- **CI jobs later tasks must extend:** format, lint, unit, race, fuzz-smoke, generated-file, documentation, security-scan, container-test. `test-config-compat` is implemented by CFG-001. `test-container` is implemented by DEP-001. `test-integration` is implemented by PERF-001 (short interop + soak/flood). `test-parity` remains fail-closed until MCP-001.
