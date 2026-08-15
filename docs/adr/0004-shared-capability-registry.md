# ADR 0004: Use one capability registry for REST and MCP

Status: Accepted
Date: 2026-08-15

## Context

Independent adapters tend to drift in schema, defaults, authorization, errors, and audit behavior.

## Decision

Declare every public application capability once and bind it to REST and MCP adapters. Generate or verify contracts and parity tests from the registry.

## Consequences

- Strong semantic parity.
- Shared authorization and mutation semantics.
- Registry design requires care to avoid a lowest-common-denominator API.
- Transport-specific envelopes remain in adapters.

## Alternatives considered

- REST-first with MCP proxying HTTP: simple but loses native MCP schemas/resources and complicates auth/error mapping.
- Independent implementations: rejected due to drift risk.

## Review triggers

Review this decision when its assumptions no longer hold, a major protocol or library change occurs, or a new requirement conflicts with an invariant.
