# Observability

Status: Proposed
Owners: Observability, Operations
Last reviewed: 2026-08-15

## Goals

- Diagnose resolution, forwarding, cache, chaos, control-plane, and state problems.
- Keep DNS request overhead low.
- Avoid sensitive or unbounded telemetry.
- Support agent-readable health and explanation.

## Structured logs

Use structured logs with stable event names. Suggested fields:

```text
timestamp level event component request_id trace_id
state_revision generation transport capability result error_code
zone_id policy_id upstream_id duration_ms
```

Do not log complete QNAMEs or client addresses by default. Optional debug modes must be time-bounded, access-controlled, and documented.

## Metrics

### DNS

- Query count by transport, client-group class, QTYPE class, resolution source, and RCODE.
- Query latency histogram.
- Parse and admission failures.
- UDP truncation and TCP fallback.
- Active TCP connections and rejected connections.

### Resolver and cache

- Exact, wildcard, authoritative-negative, overlay-fallthrough, cache-hit, cache-miss, and upstream outcomes.
- Positive and negative cache size and evictions.
- CNAME depth failures.

### Upstreams

- Exchange count, latency, timeout, transport error, RCODE, health state, and failover.
- Pool selection and circuit state if implemented.
- `denied_forward` counts queries that needed a forward and were refused (unknown/local-only client or no permitted policy). Local answers to those clients are not counted. Metrics export of this counter is OBS-001; the orchestrator increments it now.

### Chaos

- Policy match, trigger, selected outcome, skipped reason, and expiry.
- Delay histogram and active delayed requests.
- Budget saturation and clamping.
- Drops, truncations, resets, synthetic RCODEs, TTL changes, alternate answers, and cache faults.
- Emergency-disabled state.

### Control plane

- Capability calls by REST/MCP transport, result, and latency.
- Validation and compile duration.
- Revision conflicts and idempotency hits.
- Auth failures and scope denials.
- Drift and generation.

## Cardinality policy

Allowed bounded labels include configured zone ID, chaos policy ID, upstream ID, capability name, transport, RCODE, and result. Prohibited default labels include raw QNAME, raw client IP, idempotency key, actor ID, and arbitrary error text.

## Tracing

Tracing is optional and sampled. Spans may include DNS receive, local resolve, cache lookup, upstream exchange, chaos phase, state compile, and capability invocation. Sensitive names are hashed or omitted according to policy.

## Health

- Liveness: process event loop and listener health only.
- Readiness: valid active snapshot, required listeners bound, and management dependencies needed by deployment policy.
- Upstream failure does not necessarily make the service unready when local zones still work; expose degraded state separately.
- Chaos does not affect health endpoints.

## Agent-readable status

REST and MCP expose:

- Active and bootstrap revisions.
- Drift state.
- Listener status.
- Cache summary.
- Upstream health.
- Chaos enabled/emergency-disabled state.
- Active policy summary and nearest expiry.
- Recent bounded operational warnings.

## Failure modes

Telemetry backpressure must not block DNS. Use bounded queues and drop counters. Exporter failure is visible but not allowed to allocate without bound.

## Testing strategy

- Metric presence and label-policy tests.
- No-QNAME-label regression tests.
- Bounded queue tests.
- Trace redaction tests.
- Health semantics tests.
- Chaos telemetry golden tests.

## Compatibility implications

Metric names and labels are operational interfaces. Rename or semantic changes require migration notes and a deprecation window.
