# Operations and Runbooks

Status: Proposed
Owners: Operations
Last reviewed: 2026-08-15 (OBS-001 diagnostic queries)
Last reviewed: 2026-08-15 (DEP-001 SIGUSR1 CLI + graceful shutdown)

## Routine checks

Operators should monitor:

- Readiness and degraded state (`GET /v1/health/ready`, `GET /v1/status`).
- Active and bootstrap revisions and drift (`labdns_state_drifted`, Status `revisions`).
- Upstream health and timeout rate (`labdns_upstream_health`, `labdns_upstream_timeouts_total`).
- Query latency and RCODE changes (`labdns_dns_queries_total`, `labdns_dns_query_duration_seconds`).
- Cache hit rate and evictions (`labdns_cache_lookups_total`, `labdns_cache_evictions_total`).
- Active chaos policies, expiry, budget use, and emergency-disable state (`labdns_chaos_*`, Status `chaos`).
- Control-plane authorization failures (`labdns_auth_failures_total`).
- Telemetry drops (`labdns_telemetry_dropped_total`).
- Audit delivery.

Catalog: [api/metrics/v1alpha1.json](https://github.com/hilather/go-lab-dns/blob/main/api/metrics/v1alpha1.json). Semantics: [docs/09-observability.md](https://github.com/hilather/go-lab-dns/blob/main/docs/09-observability.md).

## Diagnostic queries

Prometheus-style examples (in-process scrape via `observability.Registry.WritePrometheus`):

```text
# Resolution mix (exact vs wildcard vs cache vs upstream)
sum by (source, rcode) (labdns_dns_queries_total)

# Refused forwards (unknown / local-only / no policy)
sum by (result) (labdns_dns_denied_forward_total)

# Cache effectiveness
sum(labdns_cache_lookups_total{result="hit"}) /
  clamp_min(sum(labdns_cache_lookups_total), 1)

# Unhealthy upstreams
labdns_upstream_health == 0

# Chaos under load
labdns_chaos_delayed_requests
sum by (policy_id, action) (labdns_chaos_effects_total)
labdns_chaos_emergency_disabled

# Control-plane errors
sum by (capability, result) (labdns_capability_calls_total{result="error"})

# Telemetry backpressure
sum by (reason) (labdns_telemetry_dropped_total)
```

Alert recommendations for DEP/GIT:

- `labdns_dns_denied_forward_total` rising unexpectedly (open-resolver or client-group misconfig).
- `labdns_telemetry_dropped_total` non-zero for more than a scrape interval after an export queue is attached (`EnableExport`) or a log `WithQueue` overflows. In-memory scrapes (`WritePrometheus`) do not use the export queue.
- `labdns_chaos_emergency_disabled == 1` outside a planned drill.
- Status `degraded=true` while `ready=true` (upstream outage, local zones still serve).

## Runbook: unexpected resolution

1. Call `resolve:explain` with the name, type, client group, and transport.
2. Confirm active state revision.
3. Inspect exact owner, wildcard source, closest encloser, zone mode, cache, forwarding policy, upstream, and chaos decision.
4. Compare runtime and bootstrap revisions.
5. Run an independent UDP and TCP query.
6. Review recent mutation audit events.
7. Plan a correction; do not edit live memory or container files manually.
8. Add a regression probe and test for the discovered case.

## Runbook: excessive DNS latency

1. Check active chaos delays and delayed-request budget.
2. Check upstream latency, timeouts, and failover.
3. Separate local, cache, and upstream latency metrics.
4. Verify CPU, memory, file descriptors, and TCP connection limits.
5. Emergency-disable chaos if safety is uncertain.
6. Capture a reproducible query context and add a regression or load test.

## Runbook: chaos runaway

1. Invoke emergency disable through the privileged path (`app.Service.EmergencyDisableChaos`, `POST /v1/chaos:emergency-disable`, `SIGUSR1`, or `labdns chaos emergency-disable --pid-file`). This sets the inhibit bit, stamps the active snapshot, and cancels outstanding context-aware delays (`Budgets.CancelAll`). The PID CLI sends `SIGUSR1` and does not call HTTP, so it works when management is unbound.
2. If unavailable, restart with `labdns serve --chaos-disable` or `LABDNS_CHAOS_DISABLE=1`. YAML cannot relax that startup override.
3. Confirm no new policy actions are selected (`Decide` reason `emergency_disabled`) and delayed-request count drains to zero. In-flight sleeps return promptly; new queries are not delayed.
4. Preserve audit and telemetry evidence (delay/drop/truncate/reset/RCODE counters — never raw QNAME labels).
5. Identify missing cap (`maxDelay`, `maxConcurrentDelayed`), cancellation, scope, or expiry control. Conflicting terminal transport actions are rejected at validate time.
6. Add regression tests (emergency under delayed load, cancel-releases-budget) and harden CI before re-enabling.
7. Update design and runbook documentation if behavior changed.

## Runbook: upstream outage

1. Check pool health and selected forwarding policy.
2. Confirm whether failover conditions permit another upstream (`onTimeout` / `onTransportError` / `onSERVFAIL` / `onREFUSED`; omitted bools stay false). Zero `failover.timeout` is a 500ms attempt budget (under the 2s query timeout), not unlimited.
3. Query each upstream independently from the same network namespace.
4. Avoid treating NXDOMAIN as an outage.
5. Use a temporary bounded upstream fault simulation only in a test environment.
6. Commit durable upstream changes through the deployment repository.

## Runbook: bad runtime mutation

1. Deactivate the specific policy or plan a corrective state change (`dns_change_plan` / `app.Service.Plan`).
2. If broad or uncertain, reset to bootstrap (`dns_state_reset` / `app.Service.Reset`). Reset rereads the mount, compiles, and swaps only after success. A missing or invalid file leaves the active snapshot unchanged.
3. Verify revision (`GetState`) and run `resolve` / `explain` probes.
4. Review why planning or approval did not catch the issue. Replays of the same idempotency key return the original result; a different body with that key is `idempotency_conflict`. Reset clears the in-memory idempotency LRU (default 256).
5. Add a validation rule, impact warning, regression test, or permission boundary.
6. The process never writes the bootstrap file. Persist drift via `Export` (canonical YAML/JSON + bootstrap-to-runtime operations) into the deployment repository.

## Runbook: invalid new bootstrap file

1. Do not restart a healthy process unnecessarily. A failed `labdns serve --config` start does not bind DNS. A failed `Reset` also leaves the current runtime serving.
2. Use `state:validate` / `app.Service.Validate` against the candidate document or operations. Structured `validation_failed` field violations name the path.
3. Inspect structured field and invariant errors.
4. Fix the deployment repository and rerun CI.
5. Reset or redeploy only after validation passes.

## Runbook: CI failure during release

1. Stop the release; do not tag.
2. Reproduce the failure with retained artifacts.
3. Classify product defect, test defect, environment defect, or pipeline defect.
4. Fix the cause.
5. Add regression coverage or pipeline diagnostics.
6. Rerun all required checks.
7. Record any externally visible behavior in release notes.

## Backup and recovery

There is no runtime database to back up. Recovery inputs are:

- Deployment repository.
- Pinned container image.
- Secrets and workload identity configuration.
- External audit and telemetry data.

Container recreation restores bootstrap desired state and intentionally discards runtime drift.

## Testing strategy

Exercise runbooks in staging, especially reset, invalid bootstrap, emergency chaos disable, upstream outage, and container recreation.
