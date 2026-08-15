# ADR 0003: Use ephemeral runtime state with GitOps desired state

Status: Accepted
Date: 2026-08-15

## Context

Lab deployments need easy reset and reviewable configuration. Internal durable state would create a second source of truth and complicate disaster recovery.

## Decision

Load strict YAML at startup, keep runtime state in memory, expose revisions and drift, never rewrite bootstrap, and use a separate deployment repository for durable desired state.

## Consequences

- Restart returns to Git-controlled state.
- Runtime experiments are easy to discard.
- Export and deployment-repo workflows are required.
- Multi-replica runtime mutation is not strongly consistent in the initial release.

## Alternatives considered

- Embedded database: durable but conflicts with reset and Git ownership.
- Direct Git writes from service: broad credentials and coupling.
- No runtime writes: safer but too restrictive for agents and lab experiments.

## Review triggers

Review this decision when its assumptions no longer hold, a major protocol or library change occurs, or a new requirement conflicts with an invariant.
