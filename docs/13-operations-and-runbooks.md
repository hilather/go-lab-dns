# Operations and Runbooks

Status: Proposed
Owners: Operations
Last reviewed: 2026-08-15 (CHA-001 SIGUSR1 emergency)

## Routine checks

Operators should monitor:

- Readiness and degraded state.
- Active and bootstrap revisions and drift.
- Upstream health and timeout rate.
- Query latency and RCODE changes.
- Cache hit rate and evictions.
- Active chaos policies, expiry, budget use, and emergency-disable state.
- Control-plane authorization failures.
- Audit delivery.

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

1. Invoke emergency disable through the privileged path (`app.Service.EmergencyDisableChaos`, or `SIGUSR1` to the process).
2. If unavailable, restart with `labdns serve --chaos-disable` or `LABDNS_CHAOS_DISABLE=1`.
3. Confirm no new policy actions are selected and delayed-request count drains.
4. Preserve audit and telemetry evidence.
5. Identify missing cap, cancellation, scope, or expiry control.
6. Add regression tests and harden CI before re-enabling.
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
