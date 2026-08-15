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
- `internal/dnsquery` orchestrator (`dnsserver.Handler`): classify → pre `chaos.Decide` → resolve → optional exchange → post `chaos.Decide`. CHA-002 executes delay, RCODE/TTL/answer/EDE, transport hints, and cache/upstream/pressure hooks.
- Compiled `snapshot.AccessIndex` (longest-prefix CIDR → client group) filled by `snapshot.CompileAccess`.
- `compiler.Compile` orchestrates normalize/validate, zone/forwarding/chaos/access indexes, and `sha256:` revision hashing.
- `labdns serve --config PATH` loads bootstrap YAML, compiles an immutable snapshot, and binds UDP/TCP DNS. Invalid bootstrap does not listen.
- `internal/app.Service` mutation core: `Plan`/`Apply`/`Validate`/`Export`/`Reset`, zone/record/resolve/explain queries, forwarding/cache views, in-memory audit ring, and `EmergencyDisableChaos`.
- Frozen capability registry in `internal/capabilities` covering every first-GA REST↔MCP table row. Health live/ready are REST-only (not tools). Generated manifest: [api/capabilities/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/capabilities/v1.json).
- Domain-error mapping helpers: `domainerr` → RFC 9457 `application/problem+json` status hints and MCP JSON-RPC `data`. Parity harness fails if a table row is missing or renamed.
- REST `/v1` adapter in `internal/control/rest`: routes registered from the capability registry, `application/problem+json`, loopback-only unauthenticated management (Q-AUTH), body/timeout/concurrency limits, and generated OpenAPI at [api/openapi/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/openapi/v1.json). MCP is not implemented.

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

- Compiled `snapshot.ChaosIndex` (record/wildcard/owner/zone/forwarding/pool/group/global) filled by `chaos.Compile`.
- Deterministic selector `hash-v1` (SHA-256, ten length-prefixed fields, `u0`/`u1` uniforms). Goldens in [testdata/hash-v1/vectors.json](https://github.com/hilather/go-lab-dns/blob/main/testdata/hash-v1/vectors.json) include `timeBucket: 1s` same UTC second vs next second.
- `Engine.Decide` / `Simulate` use pre-classified client-group, zone, and forwarding IDs. Simulation never sleeps, writes cache, or consumes budgets.
- Gates: enabled, start/expiry, periodic flap, every-Nth. Composition: compose / terminal / exclusive-group.
- Global and per-policy delay/concurrency budgets with reservation/release. Protected names, protected client groups, and `chaosExempt` skip execution.
- `ActivateChaos` / `DeactivateChaos` / `SetChaosExpiry` compile to `OpUpdate` + `TargetChaosActivation` via `Apply`.
- `SimulateChaos` returns explained `hash-v1` decisions.
- `EmergencyDisableChaos` sets a store-level inhibit bit that `Store.Swap` stamps onto every snapshot (apply cannot clear it). The emergency path CAS-stamps the current snapshot and does not roll back a concurrent apply. YAML `emergencyDisabled` still forces the bit on.
- `SIGUSR1` (and `labdns serve --chaos-disable` / `LABDNS_CHAOS_DISABLE=1`) set the same inhibit bit. `SIGUSR2` is ignored.
- CHA-002 executes the first-GA catalog in `internal/chaos/effects`: context-aware `fixed`/`uniform` delay (all four phases, budgets, cancel), RCODE/NODATA/EDE, TTL set/clamp/zero/jitter, alternate/omit/limit/shuffle/rotate, UDP drop/TC, TCP close/reset/hold, cache bypass/force-miss/expire/stale, upstream delay/unavailable/force/timeout/transport-error/failover/synthetic RCODE, and policy-scoped pressure. ADR 0007 malformed-wire effects are not implemented. Support matrix: [api/chaos/effects.json](https://github.com/hilather/go-lab-dns/blob/main/api/chaos/effects.json).

### REST API

- stdlib `net/http` management server (`internal/control/rest`) exposes every first-GA capability on `/v1`. Default listen address is `:8080`.
- Generated OpenAPI 3.1: [api/openapi/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/openapi/v1.json).
- Errors use `capabilities.ProblemFrom` → `application/problem+json`. Wrong method on a registered path is `method_not_allowed` (405).
- Unauthenticated remote management is denied; `127.0.0.1` / `::1` may omit a bearer token. Health live/ready remain probe-accessible.
- `POST /v1/chaos:emergency-disable` exists and shares `app.Service`; packet-level chaos still runs on the DNS path independently of REST.
- `labdns serve` does not yet bind the management listener (DEP-001).

### MCP API and protocol compatibility

- MCP tool names and `labdns://` resource templates are frozen in the shared registry. No MCP server is implemented yet.

### Configuration and schema

- Strict v1alpha1 YAML/JSON decode in `internal/config` with unknown-field rejection, refuse-forward default-deny, materialized listen/TTL/CNAME defaults, canonical JSON/YAML export, and `sha256:` revision hashing.
- Published JSON Schema at [api/jsonschema/labdns.dev.v1alpha1.json](https://github.com/hilather/go-lab-dns/blob/main/api/jsonschema/labdns.dev.v1alpha1.json).
- `selector.timeBucket` must be `>= 1s`. Empty `clientGroups` is valid (local answers, no forwarding). Non-ASCII names, wildcard NS/DNAME, CNAME coexistence/loops, and self-forwarding are rejected.
- Config migration interface stub (`Migrator`); only `labdns.dev/v1alpha1` exists.

### Deployment and operations

- `labdns serve --config PATH` is the M1 process entry: compile bootstrap YAML and serve DNS on the configured listen address (default `:5353` UDP+TCP). Management HTTP (`rest.Server`, default `:8080`) is implemented but not yet wired into `serve` (DEP-001).
- Runtime plan/apply/reset are process-local via `app.Service`. `expectedRevision` is required except privileged reset. Idempotency keys use a bounded in-memory LRU (default 256; `<=0` is not unlimited). Reset rereads the bootstrap mount and never writes it. Export is canonical YAML/JSON with no comments, plus bootstrap-to-runtime operations.

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
- REST/MCP adapters are not implemented; `app.Service` is the HTTP-less mutation surface.
- MCP adapter is not implemented; REST `/v1` is available via `rest.Server` but not yet wired into `labdns serve` (DEP-001).
- `labdns chaos emergency-disable --pid-file` is not implemented until DEP-001.
- `make test-integration`, `make test-parity`, and `make test-container` fail closed until later PRs.
- YAML configuration decode, JSON Schema, snapshot compilation, resolver, forwarder, and control-plane work have not started.
- DNS listeners require a `dnsserver.Handler`; the process does not yet serve from a compiled snapshot.
- `make test-integration`, `make test-parity`, `make test-config-compat`, and `make test-container` fail closed until later PRs.
