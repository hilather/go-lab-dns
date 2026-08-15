# ADR 0006: Pin supported MCP protocol versions

Status: Accepted
Date: 2026-08-15

## Context

MCP evolves and recent revisions changed transport and statelessness behavior. Claiming support for an unpinned latest version would make compatibility and testing ambiguous.

## Decision

Target MCP 2026-07-28 initially, record supported versions in build metadata, use the official Go SDK behind an adapter, and add a version only after conformance and parity tests pass.

## Consequences

- Behavior is reproducible.
- Protocol upgrades are explicit reviewed work.
- Supporting older versions may require compatibility code.
- Release notes must list MCP version changes.

## Alternatives considered

- Track SDK main automatically: rejected due to nondeterminism.
- Custom MCP implementation: rejected unless the official SDK cannot satisfy required behavior and an ADR replaces this one.

## Review triggers

Review this decision when its assumptions no longer hold, a major protocol or library change occurs, or a new requirement conflicts with an invariant.
