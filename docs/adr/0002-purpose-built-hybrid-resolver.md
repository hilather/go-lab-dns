# ADR 0002: Build a purpose-built hybrid resolver

Status: Accepted for initial implementation
Date: 2026-08-15

## Context

The core product is dynamic, revisioned, agent-controlled local DNS behavior with REST/MCP parity and per-entry chaos. Existing DNS servers are strong at DNS but generally center static configuration or plugin chains rather than this state and capability model.

## Decision

Build a purpose-built authoritative-local and overlay-forwarding service on a mature DNS protocol library. Do not implement DNS wire parsing from scratch and do not fork a general DNS server.

## Consequences

- The domain model can center immutable state, explainability, parity, and chaos.
- DNS correctness remains delegated to a mature wire library where appropriate.
- The project owns more resolver semantics and must maintain extensive interoperability tests.

## Alternatives considered

- CoreDNS plugin: lower initial DNS work but significant effort to make dynamic REST/MCP state the primary model.
- dnsmasq wrapper: insufficient typed state, parity, and chaos capabilities.
- Full recursive resolver: unnecessary scope and security risk.

## Review triggers

Review this decision when its assumptions no longer hold, a major protocol or library change occurs, or a new requirement conflicts with an invariant.
