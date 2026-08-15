# FWD-001: Forwarding and Cache

Status: not-started
Recommended owner: DNS forwarding agent
Dependencies: CFG-001, DNS-001, RES-001 interfaces
Exclusive ownership: `internal/forwarder`, `internal/cache`

## Goal

Implement suffix-specific upstream forwarding, UDP/TCP exchanges, health-aware failover, and bounded positive/negative caching.

## Work items

- [ ] Compile longest-suffix forwarding policy index.
- [ ] Implement ordered, round-robin, random, and health-aware pool strategies as approved for the first release.
- [ ] Implement UDP exchange and TCP retry after upstream truncation.
- [ ] Implement explicit connect, exchange, and total deadlines.
- [ ] Implement failover conditions for timeout, transport error, SERVFAIL, and REFUSED.
- [ ] Ensure NXDOMAIN does not normally fail over.
- [ ] Detect configured self-forwarding and obvious cycles.
- [ ] Implement positive and negative cache with bounded entries and TTL clamps.
- [ ] Namespace or invalidate local cache data by state revision.
- [ ] Retain source metadata for explanation.
- [ ] Add upstream health checks that cannot overload upstreams.
- [ ] Expose safe request hooks for chaos before/after upstream and cache bypass/force-miss/stale copy.

## Required tests

- [ ] Longest-suffix and default policy tests.
- [ ] UDP upstream success and TCP fallback tests.
- [ ] Timeout, transport error, SERVFAIL, REFUSED, and NXDOMAIN failover matrix.
- [ ] Self-loop configuration rejection tests.
- [ ] Positive and negative TTL tests.
- [ ] NXDOMAIN/NODATA cache distinction tests.
- [ ] Cache eviction and concurrency tests.
- [ ] Upstream cancellation and leak tests.
- [ ] Health strategy deterministic tests with fake clock.
- [ ] Packet-level integration tests with controlled fake upstreams.
- [ ] Regression tests for each forwarding/cache defect.

## Documentation updates

- [ ] Document exact pool strategies and failover defaults.
- [ ] Update DNS semantics, configuration, observability, and operations docs.
- [ ] Add release-note entry for forwarding/cache functionality.

## Acceptance criteria

- Public names resolve through configured upstreams.
- Authoritative misses are not forwarded.
- Overlay misses are forwarded.
- Cache and failover behavior are explainable and bounded.

## Handoff

Document forwarder request/result types, cache hooks, upstream identifiers, and metrics contracts.
