# OBS-001: Observability and Health

Status: not-started
Recommended owner: Observability agent
Dependencies: stable component interfaces
Exclusive ownership: `internal/observability`, metrics catalog, health status aggregation

## Goal

Add low-overhead structured logs, metrics, optional tracing, health, degraded status, and agent-readable diagnostics without sensitive or unbounded labels.

## Work items

- [ ] Define a versioned metric and event catalog.
- [ ] Instrument DNS transport, resolver, wildcard, negative answers, cache, forwarding, upstreams, chaos, state, REST, MCP, auth, and audit.
- [ ] Implement label allowlist and automated no-QNAME/no-client-IP checks.
- [ ] Implement liveness, readiness, and degraded status semantics.
- [ ] Implement build, revision, drift, upstream, cache, and chaos status views.
- [ ] Add request/trace correlation across REST/MCP and application handlers.
- [ ] Add optional sampled DNS tracing with redaction.
- [ ] Implement bounded telemetry queues and drop metrics.
- [ ] Add operational dashboards or documented queries as Markdown examples.

## Required tests

- [ ] Expected metrics are emitted for representative flows.
- [ ] Raw QNAME and client IP are not metric labels.
- [ ] Log and trace redaction tests.
- [ ] Exporter failure/backpressure does not block DNS.
- [ ] Liveness/readiness/degraded state tests.
- [ ] Chaos policy metrics are bounded by configured policy count.
- [ ] Metric catalog compatibility diff tests.
- [ ] Regression test for every observability defect.

## Documentation updates

- [ ] Publish metric names, types, labels, and semantics.
- [ ] Update runbooks with diagnostic queries.
- [ ] Document privacy and debug-mode controls.
- [ ] Add release-note entry for observability changes.

## Acceptance criteria

- Operators can diagnose exact/wildcard/cache/upstream/chaos source without high-cardinality metrics.
- Telemetry failure cannot materially impair DNS.
- Health endpoints are unaffected by chaos.

## Handoff

Provide metric catalog and alert recommendations for DEP/GIT tasks.
