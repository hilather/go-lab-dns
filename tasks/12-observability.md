# OBS-001: Observability and Health

Status: complete
Recommended owner: Observability agent
Dependencies: stable component interfaces
Exclusive ownership: `internal/observability`, metrics catalog, health status aggregation

## Goal

Add low-overhead structured logs, metrics, optional tracing, health, degraded status, and agent-readable diagnostics without sensitive or unbounded labels.

## Work items

- [x] Define a versioned metric and event catalog.
- [x] Instrument DNS transport, resolver, wildcard, negative answers, cache, forwarding, upstreams, chaos, state, REST, MCP, auth, and audit.
- [x] Implement label allowlist and automated no-QNAME/no-client-IP checks.
- [x] Implement liveness, readiness, and degraded status semantics.
- [x] Implement build, revision, drift, upstream, cache, and chaos status views.
- [x] Add request/trace correlation across REST/MCP and application handlers.
- [x] Add optional sampled DNS tracing with redaction.
- [x] Implement bounded telemetry queues and drop metrics.
- [x] Add operational dashboards or documented queries as Markdown examples.

## Required tests

- [x] Expected metrics are emitted for representative flows.
- [x] Raw QNAME and client IP are not metric labels.
- [x] Log and trace redaction tests.
- [x] Exporter failure/backpressure does not block DNS.
- [x] Liveness/readiness/degraded state tests.
- [x] Chaos policy metrics are bounded by configured policy count.
- [x] Metric catalog compatibility diff tests.
- [x] Regression test for every observability defect.

## Documentation updates

- [x] Publish metric names, types, labels, and semantics.
- [x] Update runbooks with diagnostic queries.
- [x] Document privacy and debug-mode controls.
- [x] Add release-note entry for observability changes.

## Acceptance criteria

- Operators can diagnose exact/wildcard/cache/upstream/chaos source without high-cardinality metrics.
- Telemetry failure cannot materially impair DNS.
- Health endpoints are unaffected by chaos.

## Handoff

Provide metric catalog and alert recommendations for DEP/GIT tasks.

Catalog: `api/metrics/v1alpha1.json`. Alert starters live in `docs/13-operations-and-runbooks.md`.
