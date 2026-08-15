# ADR 0001: Use Go for the service

Status: Accepted for initial implementation
Date: 2026-08-15

## Context

LabDNS combines a latency-sensitive UDP/TCP DNS data plane, concurrent upstream exchanges, HTTP management, MCP, immutable runtime state, container deployment, race testing, and fuzzing.

## Decision

Implement the service in Go. Use mature DNS and official MCP libraries behind internal adapters. Prefer the standard library for HTTP and concurrency where practical.

## Consequences

- A single static binary is easy to deploy.
- Go concurrency and context cancellation fit DNS and bounded delay behavior.
- Race detection and fuzzing support hardening.
- Library adapters reduce lock-in.
- Contributors must follow Go memory, cancellation, and error-handling discipline.

## Alternatives considered

- Rust: strong safety and performance, but higher implementation complexity for the initial team.
- TypeScript or Python: productive control planes, but less suitable for the combined DNS data plane and minimal static container.
- Java: mature networking but heavier runtime and image footprint for this use case.

## Review triggers

Review this decision when its assumptions no longer hold, a major protocol or library change occurs, or a new requirement conflicts with an invariant.
