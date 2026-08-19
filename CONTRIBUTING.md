# Contributing

## Toolchain

- Go 1.26 (`go1.26.x`). The module path is `github.com/hilather/go-lab-dns`.
- Operator console: Node **22.14.0**, npm with `web/package-lock.json`.
- `make lint` runs `go vet` and golangci-lint v2.12.2.
- `make security-scan` runs `golang.org/x/vuln/cmd/govulncheck@v1.1.4`.

See the [Build and test](https://github.com/hilather/go-lab-dns/blob/main/README.md) section of the README for the required Make targets. Local equivalents of every required CI job also include `make test-changelog`, `make test-parity`, `make test-config-compat`, `make web-test`, and `make web-build`. Compare public surfaces with `make release-diff FROM=<prev> TO=HEAD`. Tag notes live at `docs/releases/<tag>.md`. The 1.0.0-rc.2 notes are [docs/releases/v1.0.0-rc.2.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.0.0-rc.2.md); see [docs/14-release-engineering.md](https://github.com/hilather/go-lab-dns/blob/main/docs/14-release-engineering.md).

## Development workflow

1. Choose or create a tracked task.
2. Read the normative design documents and relevant ADRs.
3. Add or update tests that express the intended behavior.
4. Implement the smallest coherent change.
5. Update all affected documentation.
6. Run local CI-equivalent targets.
7. Submit a reviewable pull request with risk, test, compatibility, and release-note information.

## Pull request requirements

Every pull request must state:

- Problem and intended outcome.
- Scope and explicit non-scope.
- Architectural invariants touched.
- Security and abuse considerations.
- Test evidence, including regression tests.
- REST/MCP parity impact.
- Configuration and compatibility impact.
- Documentation changed.
- Release-note entry or explicit reason that no externally observable behavior changed.
- Rollback strategy.

## Change sizing

Prefer small vertical slices that compile and pass tests. Do not merge partial public APIs, undocumented schema fields, or disabled tests as placeholders. Feature flags may be used only when their ownership, default, removal plan, and test matrix are documented.

## Commit and review discipline

- Keep generated changes separate when practical.
- Do not mix broad refactors with protocol changes unless necessary.
- Require review from owners of DNS semantics, API/MCP parity, security, and release engineering when those areas change.
- Resolve review findings in code, tests, and documentation rather than only in comments.

## Backward compatibility

Follow `docs/16-compatibility-and-versioning.md`. Breaking changes require an ADR, migration instructions, release-note treatment, and the appropriate version increment.
