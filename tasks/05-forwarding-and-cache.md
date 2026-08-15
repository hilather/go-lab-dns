# FWD-001: Forwarding and Cache

Status: in-progress (FWD-001 data plane; AccessIndex/bootstrap serve is STA-001 slice A)
Recommended owner: DNS forwarding agent
Dependencies: CFG-001, DNS-001, RES-001 interfaces
Exclusive ownership: `internal/forwarder`, `internal/cache`

## Goal

Implement suffix-specific upstream forwarding, UDP/TCP exchanges, health-aware failover, and bounded positive/negative caching.

## Work items

- [x] Compile longest-suffix forwarding policy index.
- [x] Implement ordered, round-robin, random, and health-aware pool strategies as approved for the first release.
- [x] Implement UDP exchange and TCP retry after upstream truncation.
- [x] Implement explicit connect, exchange, and total deadlines.
- [x] Implement failover conditions for timeout, transport error, SERVFAIL, and REFUSED.
- [x] Ensure NXDOMAIN does not normally fail over.
- [x] Detect configured self-forwarding and obvious cycles.
- [x] Implement positive and negative cache with bounded entries and TTL clamps.
- [x] Namespace or invalidate local cache data by state revision.
- [x] Retain source metadata for explanation.
- [x] Add upstream health checks that cannot overload upstreams.
- [x] Expose safe request hooks for chaos before/after upstream and cache bypass/force-miss/stale copy.

## Required tests

- [x] Longest-suffix and default policy tests.
- [x] UDP upstream success and TCP fallback tests.
- [x] Timeout, transport error, SERVFAIL, REFUSED, and NXDOMAIN failover matrix.
- [x] Self-loop configuration rejection tests.
- [x] Positive and negative TTL tests.
- [x] NXDOMAIN/NODATA cache distinction tests.
- [x] Cache eviction and concurrency tests.
- [x] Upstream cancellation and leak tests.
- [x] Health strategy deterministic tests with fake clock.
- [x] Packet-level integration tests with controlled fake upstreams.
- [ ] Regression tests for each forwarding/cache defect.

## Documentation updates

- [x] Document exact pool strategies and failover defaults.
- [x] Update DNS semantics, configuration, observability, and operations docs.
- [x] Add release-note entry for forwarding/cache functionality.

## Acceptance criteria

- Public names resolve through configured upstreams.
- Authoritative misses are not forwarded.
- Overlay misses are forwarded.
- Cache and failover behavior are explainable and bounded.

## Handoff

Document forwarder request/result types, cache hooks, upstream identifiers, and metrics contracts.
