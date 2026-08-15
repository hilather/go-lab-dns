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
- Compiled `snapshot.ZoneIndex` (existence tree, RRsets, wildcards, longest-suffix `Select`) filled by `resolver.Compile`. `Resolve` consumes a pre-selected zone ID.
- Compiled `snapshot.ForwardingIndex` (longest-suffix policies, default `.`) filled by `forwarder.Compile`. `Exchange` consumes a pre-selected policy ID.
- Process-scoped positive/negative `internal/cache` namespaced by snapshot revision, with TTL clamps, LRU eviction, and chaos lookup hooks.
- `internal/dnsquery` orchestrator (`dnsserver.Handler`): classify → resolve → optional exchange. No chaos `Decide` yet.
- Compiled `snapshot.AccessIndex` (longest-prefix CIDR → client group) filled by `snapshot.CompileAccess`.
- `compiler.Compile` orchestrates normalize/validate, zone/forwarding/access indexes, and `sha256:` revision hashing. Chaos compile is a no-op until CHA-001.
- `labdns serve --config PATH` loads bootstrap YAML, compiles an immutable snapshot, and binds UDP/TCP DNS. Invalid bootstrap does not listen.

### Changed

- None.

### Fixed

- None.

### Removed or deprecated

- None.

### Security

- Data-plane refuse-forward: unmatched clients are not an open recursive resolver. Local answers still serve.

### DNS behavior

- UDP and TCP listeners admit a single IN class QUERY, echo ID/question, set QR, and apply FORMERR/NOTIMP/BADVERS as documented in [docs/02-dns-semantics.md](https://github.com/hilather/go-lab-dns/blob/main/docs/02-dns-semantics.md).
- Local resolver (`internal/resolver`): exact RRsets, RFC 4592 closest-encloser wildcards, empty non-terminals, bounded CNAME (default depth 8), authoritative NXDOMAIN/NODATA+SOA, overlay fallthrough. Overlay CNAME may terminate in a forwarded name (`Fallthrough=true`); the resolver does not forward. AA only on authoritative local/negative answers; AD never set; CD cleared locally. Zero `CNAMEDepth` falls back to 8, not unlimited.
- Suffix forwarding (`internal/forwarder`): ordered / round-robin / random / health-aware pools; UDP exchange with optional TCP retry after TC; failover only when `FailoverSpec` bools are set. NXDOMAIN does not fail over. Zero `timeout` is a 500ms per-attempt budget (250ms connect), stacked under the 2s query timeout, not unlimited. No `/etc/resolv.conf` fallback. Forwarded answers: AA=0, AD=0, CD pass-through.
- Refuse-forward: unknown or `AllowForward=false` clients get local answers with RA=0 and never hit upstreams. REFUSED only when there is no local path. Empty `clientGroups` serves local zones and forwards nothing. Pack-sample `ns1.lab.example.net` from 127.0.0.1 succeeds; a forward-only name from an unmatched IP is REFUSED with zero upstream packets.

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

- `labdns serve --config PATH` is the M1 process entry: compile bootstrap YAML and serve DNS on the configured listen address (default `:5353` UDP+TCP). Management HTTP is not bound.

### Observability

- None.

### Compatibility and migration

- None.

### Known limitations

- Snapshot compilation, DNS wire, resolver, forwarder, and control-plane work have not started.
- Forwarder, cache, snapshot bootstrap serve, and control-plane work have not started.
- DNS listeners require a `dnsserver.Handler`; the process does not yet serve from a compiled snapshot (no `dnsquery` orchestrator yet).
- Snapshot bootstrap serve (`compiler.Compile`, `AccessIndex` fill, `cmd/labdns serve`) has not started; `dnsquery` is constructed in tests against a hand-built `snapshot.Store`.
- Plan/apply/export/reset and REST/MCP are not implemented; M1 serves only the compiled bootstrap snapshot.
- Chaos `Decide` is not wired; cache/exchange hooks exist but are unused on the live path.
- `make test-integration`, `make test-parity`, and `make test-container` fail closed until later PRs.
- YAML configuration decode, JSON Schema, snapshot compilation, resolver, forwarder, and control-plane work have not started.
- DNS listeners require a `dnsserver.Handler`; the process does not yet serve from a compiled snapshot.
- `make test-integration`, `make test-parity`, `make test-config-compat`, and `make test-container` fail closed until later PRs.
