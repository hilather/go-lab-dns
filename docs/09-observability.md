# Observability

Status: Proposed
Owners: Observability, Operations
Last reviewed: 2026-08-15 (OBS-001 catalog, Status DTO, health)

## Goals

- Diagnose resolution, forwarding, cache, chaos, control-plane, and state problems.
- Keep DNS request overhead low.
- Avoid sensitive or unbounded telemetry.
- Support agent-readable health and explanation.

## Structured logs

Use structured JSON logs with stable event names. Fields:

```text
timestamp level event component request_id trace_id
state_revision generation transport capability result error_code
zone_id policy_id upstream_id duration_ms
```

Do not log complete QNAMEs or client addresses by default. Optional debug (`spec.observability.logQNAME`) must be time-bounded, access-controlled, and documented. The same gate may include a client address; it is off unless that flag is set.

Event names are frozen in [`api/metrics/v1alpha1.json`](https://github.com/hilather/go-lab-dns/blob/main/api/metrics/v1alpha1.json) (`events[]`).

## Metrics

The versioned catalog is [`api/metrics/v1alpha1.json`](https://github.com/hilather/go-lab-dns/blob/main/api/metrics/v1alpha1.json) (`labdns.dev/metrics/v1alpha1`). Go source of truth: `internal/observability`. `make generate` / `make verify-generated` keep the artifact current.

Rename or semantic change of a metric name or label requires migration notes and a deprecation window.

### DNS

| Name | Kind | Labels |
|---|---|---|
| `labdns_dns_admitted_total` | counter | `transport` |
| `labdns_dns_queries_total` | counter | `transport`, `client_group_class`, `qtype_class`, `source`, `rcode` |
| `labdns_dns_query_duration_seconds` | histogram | `transport`, `source` |
| `labdns_dns_parse_total` | counter | `result` |
| `labdns_dns_admission_total` | counter | `result`, `rcode` |
| `labdns_dns_responses_total` | counter | `transport`, `rcode`, `action` |
| `labdns_dns_tcp_events_total` | counter | `event` |
| `labdns_dns_denied_forward_total` | counter | `result` |

`denied_forward` counts queries that needed a forward and were refused (unknown/local-only client or no permitted policy). Local answers to those clients are not counted.

### Resolver and cache

| Name | Kind | Labels |
|---|---|---|
| `labdns_resolver_outcomes_total` | counter | `source`, `zone_id` |
| `labdns_resolver_cname_depth_failures_total` | counter | `zone_id` |
| `labdns_cache_lookups_total` | counter | `result` |
| `labdns_cache_entries` | gauge | `kind` |
| `labdns_cache_evictions_total` | counter | — |

`source` is `exact` / `wildcard` / `negative` / `fallthrough` / `upstream` / `cache`.

### Upstreams

| Name | Kind | Labels |
|---|---|---|
| `labdns_upstream_exchanges_total` | counter | `upstream_id`, `rcode`, `result` |
| `labdns_upstream_exchange_duration_seconds` | histogram | `upstream_id` |
| `labdns_upstream_timeouts_total` | counter | `upstream_id` |
| `labdns_upstream_transport_errors_total` | counter | `upstream_id` |
| `labdns_upstream_health` | gauge | `upstream_id` |
| `labdns_upstream_failovers_total` | counter | `upstream_id` |

### Chaos

| Name | Kind | Labels |
|---|---|---|
| `labdns_chaos_policy_matches_total` | counter | `policy_id`, `result` |
| `labdns_chaos_policy_triggers_total` | counter | `policy_id`, `outcome` |
| `labdns_chaos_policy_skips_total` | counter | `policy_id`, `reason` |
| `labdns_chaos_delay_seconds` | histogram | `policy_id` |
| `labdns_chaos_delayed_requests` | gauge | — |
| `labdns_chaos_budget_saturations_total` | counter | `policy_id` |
| `labdns_chaos_effects_total` | counter | `policy_id`, `action` |
| `labdns_chaos_emergency_disabled` | gauge | — |

`policy_id` is bounded by configured policy count. The registry also hard-caps series per metric name (256). CHA-002 process counters on `chaos.Engine.Stats` remain available without QNAME/client-IP labels.

### Control plane

| Name | Kind | Labels |
|---|---|---|
| `labdns_capability_calls_total` | counter | `capability`, `transport`, `result` |
| `labdns_capability_duration_seconds` | histogram | `capability`, `transport` |
| `labdns_state_compile_duration_seconds` | histogram | — |
| `labdns_state_validation_duration_seconds` | histogram | — |
| `labdns_state_revision_conflicts_total` | counter | — |
| `labdns_state_idempotency_hits_total` | counter | — |
| `labdns_auth_failures_total` | counter | `result` |
| `labdns_state_generation` | gauge | — |
| `labdns_state_drifted` | gauge | — |
| `labdns_telemetry_dropped_total` | counter | `reason` |

Live increments today: DNS admitted/queries/duration/parse/admission/responses/tcp/denied_forward, resolver outcomes, cache lookups, chaos match/trigger/skip/effects, capability calls/duration, auth failures, state generation/drifted, chaos emergency, telemetry dropped.

Catalog-only (not yet live-incremented): CNAME depth failures, cache entries/evictions, upstream exchange/timeout/transport/health/failover, chaos delay histogram / delayed-requests / budget, compile/validate duration, revision conflicts, idempotency hits.

## Cardinality policy

Allowed bounded labels include configured zone ID, chaos policy ID, upstream ID, capability name, transport, RCODE, result, resolution source, QTYPE class, and client-group class (`known` / `unknown` / `local_only`). Prohibited default labels include raw QNAME, raw client IP, idempotency key, actor ID, and arbitrary error text.

Automated checks: catalog rows cannot declare forbidden labels; `Registry.Inc` drops samples that include them and increments `labdns_telemetry_dropped_total{reason="forbidden_label"}`.

## Tracing

Tracing is optional and sampled (`observability.Tracer`). Spans may include DNS receive, local resolve, cache lookup, upstream exchange, chaos phase, state compile, and capability invocation. Sensitive names are hashed (`sha256:` + 8 bytes) or omitted.

Request/trace correlation uses `X-Request-ID` and `X-Trace-ID` on REST and context keys shared with the application layer.

## Health

- Liveness: process serve loop only (`GET /v1/health/live` is `!closed` unless `Config.Live` overrides). Listener bind state is not part of liveness.
- Readiness: valid active snapshot and required listeners bound (`GET /v1/health/ready`). Driven by `app.Status.Ready`.
- Upstream failure does not make the service unready when local zones still work; `Status.Degraded` is set instead.
- Chaos does not affect health endpoints or `Ready`/`Degraded`. Emergency-disable is an informational Status warning only.

## Agent-readable status

`GET /v1/status` / `dns_status_get` / `labdns://status` return one `app.Status`:

- Build version.
- Ready / degraded.
- Active and bootstrap revisions, generation, drift, loaded-at.
- Listener configured addresses.
- Cache summary.
- Upstream health.
- Chaos enabled/emergency-disabled, active policy count, nearest expiry.
- Recent bounded operational warnings (`state_drifted`, `upstream_unhealthy`, `chaos_emergency_disabled`, `telemetry_dropped`, `listener_unbound`, `cache_near_capacity`, `no_active_snapshot`).

## Failure modes

Telemetry backpressure must not block DNS. Use bounded queues (`DefaultQueueSize` = 1024) and drop counters. Exporter failure is visible (`labdns_telemetry_dropped_total`) but not allowed to allocate without bound (`MaxSeriesPerMetric` = 256).

## Testing strategy

- Metric presence and label-policy tests.
- No-QNAME-label regression tests.
- Bounded queue tests.
- Trace redaction tests.
- Health semantics tests.
- Chaos telemetry series-cap tests.
- Catalog compatibility (`api/metrics/v1alpha1.json` vs `RenderCatalog`).

## Compatibility implications

Metric names and labels are operational interfaces. Rename or semantic changes require migration notes and a deprecation window.
