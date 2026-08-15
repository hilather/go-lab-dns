# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- Initial architecture and implementation plan.
- Repository foundation: Apache-2.0 license, Go 1.26 module `github.com/hilather/go-lab-dns`, package tree, Makefile, and fail-closed CI skeleton.
- Canonical domain types in `internal/model`: `State`/`Spec`, zones, records, forwarding, chaos policies, `Operation`/`Target`, and `Query`/`Result`.
- Stable domain error catalog in `internal/domainerr` matching [docs/17-error-model.md](https://github.com/hilather/go-lab-dns/blob/main/docs/17-error-model.md).
- Immutable `snapshot.Snapshot` shells and a working atomic `snapshot.Store` (`Load`/`Swap`/`Bootstrap`/`Previous`).

### Changed

- None.

### Fixed

- None.

### Removed or deprecated

- None.

### Security

- None.

### DNS behavior

- None.

### Chaos behavior

- None.

### REST API

- None.

### MCP API and protocol compatibility

- None.

### Configuration and schema

- v1alpha1 Go types are frozen for `labdns.dev/v1alpha1` (`kind: LabDNS`). User-supplied IDs only. `access.unknownClient` is `refuse-forward`. CNAME depth default is 8. YAML decode, default materialization, and JSON Schema remain later work.

### Deployment and operations

- None.

### Observability

- None.

### Compatibility and migration

- None.

### Known limitations

- YAML configuration decode, JSON Schema, snapshot compilation, DNS wire, resolver, forwarder, and control-plane work have not started.
- `make test-integration`, `make test-parity`, `make test-config-compat`, and `make test-container` fail closed until later PRs.
