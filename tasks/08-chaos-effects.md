# CHA-002: Chaos Effects

Status: not-started
Recommended owner: DNS chaos effects agent
Dependencies: CHA-001, DNS-001, RES-001, FWD-001
Exclusive ownership: effect execution and integration adapters

## Goal

Implement the initial safe chaos effect catalog, including per-DNS-entry network delay, while preserving resource bounds and DNS correctness.

## Work items

### Delay

- [ ] Implement context-aware fixed delay.
- [ ] Implement context-aware uniform delay.
- [ ] Support before-resolution, before-upstream, after-upstream, and before-response phases.
- [ ] Enforce max delay and concurrent delayed-request budgets.

### DNS response effects

- [ ] Implement SERVFAIL, REFUSED, NXDOMAIN, NODATA, FORMERR, and NOTIMP injection with safety-class validation.
- [ ] Implement optional EDE annotation.
- [ ] Implement fixed, clamped, zero, and jittered TTL.
- [ ] Implement alternate answers with CIDR/suffix allowlists.
- [ ] Implement value omission, answer limit, deterministic shuffle, rotation, and weighted subset.

### Transport effects

- [ ] Implement UDP silent drop.
- [ ] Implement UDP forced truncation.
- [ ] Implement bounded TCP no-response then close.
- [ ] Implement TCP close and reset.

### Cache and upstream effects

- [ ] Implement cache bypass and force miss.
- [ ] Implement safe stale-copy serving if available.
- [ ] Implement upstream delay, unavailable, forced selection, timeout, transport error, and failover.
- [ ] Implement policy-scoped rate/concurrency response behavior.

### Lifecycle

- [ ] Cancel delayed effects on query cancellation, shutdown, reset, or emergency disable where applicable.
- [ ] Emit metrics and audit/explanation data without raw QNAME labels.

## Required tests

- [ ] Per-exact-record fixed delay timing test.
- [ ] Per-wildcard-record uniform delay bounds test.
- [ ] Cancellation releases timer and budget.
- [ ] Maximum concurrent delayed requests cannot be exceeded.
- [ ] RCODE/NODATA packet correctness tests.
- [ ] EDE packet tests.
- [ ] TTL boundaries and no-overflow tests.
- [ ] Alternate-address allowlist and CNAME-loop tests.
- [ ] Partial-answer immutability tests.
- [ ] UDP drop and truncation end-to-end tests.
- [ ] Client retries over TCP after forced truncation.
- [ ] TCP close/reset/no-response leak tests.
- [ ] Cache and upstream effect tests with fake upstreams.
- [ ] Emergency disable under high delayed load.
- [ ] Race and soak tests.
- [ ] Regression test for every effect defect.

## Documentation updates

- [ ] Update exact effect schema, defaults, limits, and examples.
- [ ] Update operations runbook for runaway chaos.
- [ ] Update observability metrics and error codes.
- [ ] Add release-note entries for each shipped effect.

## Acceptance criteria

- All effects in `docs/03-chaos-engine.md` marked for initial release pass tests.
- Per-entry delay works for exact and synthesized wildcard RRsets.
- No effect can hang beyond global deadlines or affect management endpoints.
- Base and final behavior are visible in explanation.

## Handoff

Provide an effect support manifest for REST/MCP capabilities and release-diff tooling.
