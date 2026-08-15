# ADR 0005: Embed a bounded DNS chaos engine

Status: Accepted
Date: 2026-08-15

## Context

The lab requires per-entry delay and other DNS fault behaviors. External network emulators cannot easily express RRset, wildcard-source, RCODE, TTL, cache, or forwarding semantics and are harder for agents to explain.

## Decision

Implement a domain-aware chaos engine with stable policy IDs, deterministic selectors, weighted outcomes, fixed action phases, global caps, expiry, protected objects, simulation, audit, and emergency disable.

## Consequences

- Rich per-entry testing is possible.
- Faults are explainable and Git-controlled.
- The implementation must rigorously test timers, cancellation, resource budgets, and policy conflicts.
- Chaos privileges are separated from normal DNS editing.

## Alternatives considered

- Only use `tc`/netem: useful for network faults but not DNS-semantic faults.
- Sidecar proxy: additional hop and still limited semantic visibility.
- Unbounded inline sleeps: rejected as unsafe and untestable.

## Review triggers

Review this decision when its assumptions no longer hold, a major protocol or library change occurs, or a new requirement conflicts with an invariant.
