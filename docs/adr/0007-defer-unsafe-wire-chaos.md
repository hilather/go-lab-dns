# ADR 0007: Defer arbitrary malformed-wire chaos

Status: Accepted
Date: 2026-08-15

## Context

Malformed compression pointers, invalid lengths, mismatched IDs, and arbitrary packet bytes are useful for dedicated resolver fuzzing but substantially increase security and reliability risk in a production DNS process.

## Decision

The initial LabDNS process emits syntactically valid DNS messages only. Unsafe wire fuzzing may later be provided by a separately isolated companion with no management or forwarding credentials.

## Consequences

- Main-service attack surface remains smaller.
- Common resilience tests still use delay, drop, truncation, reset, RCODE, TTL, and answer changes.
- Dedicated client parser fuzzing is deferred.

## Alternatives considered

- Include an unsafe mode in the main service: rejected.
- Ignore malformed-client testing entirely: rejected; use external fuzz harnesses and consider a future companion.

## Review triggers

Review this decision when its assumptions no longer hold, a major protocol or library change occurs, or a new requirement conflicts with an invariant.
