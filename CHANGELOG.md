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

- Strict v1alpha1 YAML/JSON decode in `internal/config` with unknown-field rejection, refuse-forward default-deny, materialized listen/TTL/CNAME defaults, canonical JSON/YAML export, and `sha256:` revision hashing.
- Published JSON Schema at [api/jsonschema/labdns.dev.v1alpha1.json](https://github.com/hilather/go-lab-dns/blob/main/api/jsonschema/labdns.dev.v1alpha1.json).
- `selector.timeBucket` must be `>= 1s`. Empty `clientGroups` is valid (local answers, no forwarding). Non-ASCII names, wildcard NS/DNAME, CNAME coexistence/loops, and self-forwarding are rejected.
- Config migration interface stub (`Migrator`); only `labdns.dev/v1alpha1` exists.

### Deployment and operations

- None.

### Observability

- None.

### Compatibility and migration

- None.

### Known limitations

- Snapshot compilation, DNS wire, resolver, forwarder, and control-plane work have not started.
- `make test-integration`, `make test-parity`, and `make test-container` fail closed until later PRs.
