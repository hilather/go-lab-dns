# CHA-002: Chaos Effects

Status: done
Recommended owner: DNS chaos effects agent
Dependencies: CHA-001, DNS-001, RES-001, FWD-001
Exclusive ownership: effect execution and integration adapters

## Goal

Implement the initial safe chaos effect catalog, including per-DNS-entry network delay, while preserving resource bounds and DNS correctness.

## Work items

### Delay

- [x] Implement context-aware fixed delay.
- [x] Implement context-aware uniform delay.
- [x] Support before-resolution, before-upstream, after-upstream, and before-response phases.
- [x] Enforce max delay and concurrent delayed-request budgets.

### DNS response effects

- [x] Implement SERVFAIL, REFUSED, NXDOMAIN, NODATA, FORMERR, and NOTIMP injection with safety-class validation.
- [x] Implement optional EDE annotation.
- [x] Implement fixed, clamped, zero, and jittered TTL.
- [x] Implement alternate answers with CIDR/suffix allowlists.
- [x] Implement value omission, answer limit, deterministic shuffle, rotation, and weighted subset.

### Transport effects

- [x] Implement UDP silent drop.
- [x] Implement UDP forced truncation.
- [x] Implement bounded TCP no-response then close.
- [x] Implement TCP close and reset.

### Cache and upstream effects

- [x] Implement cache bypass and force miss.
- [x] Implement safe stale-copy serving if available.
- [x] Implement upstream delay, unavailable, forced selection, timeout, transport error, and failover.
- [x] Implement policy-scoped rate/concurrency response behavior.

### Lifecycle

- [x] Cancel delayed effects on query cancellation, shutdown, reset, or emergency disable where applicable.
- [x] Emit metrics and audit/explanation data without raw QNAME labels.

## Required tests

- [x] Per-exact-record fixed delay timing test.
- [x] Per-wildcard-record uniform delay bounds test.
- [x] Cancellation releases timer and budget.
- [x] Maximum concurrent delayed requests cannot be exceeded.
- [x] RCODE/NODATA packet correctness tests.
- [x] EDE packet tests.
- [x] TTL boundaries and no-overflow tests.
- [x] Alternate-address allowlist and CNAME-loop tests.
- [x] Partial-answer immutability tests.
- [x] UDP drop and truncation end-to-end tests.
- [x] Client retries over TCP after forced truncation.
- [x] TCP close/reset/no-response leak tests.
- [x] Cache and upstream effect tests with fake upstreams.
- [x] Emergency disable under high delayed load.
- [x] Race and soak tests.
- [x] Regression test for every effect defect.

## Documentation updates

- [x] Update exact effect schema, defaults, limits, and examples.
- [x] Update operations runbook for runaway chaos.
- [x] Update observability metrics and error codes.
- [x] Add release-note entries for each shipped effect.

## Acceptance criteria

- All effects in `docs/03-chaos-engine.md` marked for initial release pass tests.
- Per-entry delay works for exact and synthesized wildcard RRsets.
- No effect can hang beyond global deadlines or affect management endpoints.
- Base and final behavior are visible in explanation.

## Handoff

Provide an effect support manifest for REST/MCP capabilities and release-diff tooling.
