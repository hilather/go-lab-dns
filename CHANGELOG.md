# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- Initial architecture and implementation plan.
- Repository foundation: Apache-2.0 license, Go 1.26 module `github.com/hilather/go-lab-dns`, package tree, Makefile, and fail-closed CI skeleton.
- Canonical domain types in `internal/model`: `State`/`Spec`, zones, records, forwarding, chaos policies, `Operation`/`Target`, and `Query`/`Result`.
- Stable domain error catalog in `internal/domainerr` matching [docs/17-error-model.md](https://github.com/hilather/go-lab-dns/blob/main/docs/17-error-model.md).
- Immutable `snapshot.Snapshot` shells and a working atomic `snapshot.Store` (`Load`/`Swap`/`Bootstrap`/`Previous`).
- DNS UDP/TCP listeners in `internal/dnsserver` with admission limits, TCP framing/deadlines, graceful shutdown, and chaos transport hints (`send`/`drop`/`truncate`/`tcp-close`/`tcp-reset`/`hold-then-close`).
- `github.com/miekg/dns v1.1.72` pinned behind `internal/dnswire`; library types do not escape the adapter.

### Changed

- None.

### Fixed

- None.

### Removed or deprecated

- None.

### Security

- None.

### DNS behavior

- UDP and TCP listeners admit a single IN class QUERY, echo ID/question, set QR, and apply FORMERR/NOTIMP/BADVERS as documented in [docs/02-dns-semantics.md](https://github.com/hilather/go-lab-dns/blob/main/docs/02-dns-semantics.md). Resolver answers are still a stub until RES-001.

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
- YAML configuration decode, JSON Schema, snapshot compilation, resolver, forwarder, and control-plane work have not started.
- DNS listeners require a `dnsserver.Handler`; the process does not yet serve from a compiled snapshot.
- `make test-integration`, `make test-parity`, `make test-config-compat`, and `make test-container` fail closed until later PRs.
