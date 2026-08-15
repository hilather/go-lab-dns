# Parallelization Plan

Status: Proposed
Last reviewed: 2026-08-15

## Principle

Parallelize around stable interfaces, not shared files. Domain types, source schemas, capability registry, error catalog, and generated contracts are high-conflict surfaces and require an explicit owner.

## Early lanes

After FND-001 establishes structure:

### Lane A: Configuration contract

- CFG-001 owns model and schema.
- Other lanes consume an agreed interface or fixture branch.
- No other lane changes source schema without coordination.

### Lane B: DNS transport

- DNS-001 can proceed against small query/response interfaces.
- It must not invent resolver semantics.

### Lane C: CI/documentation infrastructure

- REL-001 can add generic checks early but must not finalize diff tools until schemas exist.

## Core lanes after config freeze

- RES-001 local resolver.
- FWD-001 forwarding/cache, using resolver fallthrough/result interfaces.
- STA-001 state orchestration, using compiler interfaces.
- OBS-001 catalog design and non-invasive hooks.

Avoid simultaneous edits to compiler ownership. Assign zone compilation to RES-001, forwarder compilation to FWD-001, chaos compilation to CHA-001, and top-level orchestration to STA-001.

## Control-plane lanes

After the shared capability registry and domain errors are frozen:

- API-001 and MCP-001 can run in parallel.
- One owner controls capability declarations and generated schema merges.
- Each adapter contributes parity fixtures but cannot change shared semantics independently.

## Chaos lanes

- CHA-001 must finish selector, phase, budget, and action-plan contracts first.
- CHA-002 can then split by effect family only if each sub-agent owns separate files and uses the same action interface:
  - delay/schedule execution;
  - response/TTL/answer mutation;
  - UDP/TCP transport effects;
  - cache/upstream effects.
- A single integrator owns cross-effect conflict tests and emergency disable.

## Security and deployment lanes

- SEC-001 can define policy and tests early, then integrate after adapters exist.
- DEP-001 can build a stub image early and finalize after server wiring.
- GIT-001 starts after CLI/schema stability.
- PERF-001 starts harness design early but records baselines only on a feature-complete candidate.

## Merge order

Recommended integration order:

```text
foundation
 -> model/schema
 -> DNS transport
 -> resolver
 -> forwarding/cache
 -> snapshot state
 -> capability registry
 -> REST and MCP
 -> chaos core
 -> chaos effects
 -> security/audit
 -> observability
 -> CLI/container
 -> deployment examples
 -> release automation finalization
 -> performance/interoperability
 -> GA hardening
```

Some branches may develop earlier, but merge order protects shared contracts.

## Coordination checkpoints

Hold a design checkpoint before changing:

- Stable IDs.
- Canonical name rules.
- Resolution result shape.
- Snapshot compiler interface.
- Domain error schema.
- Capability names.
- Chaos action phase ordering.
- Deterministic hash inputs.
- MCP protocol version.
- Public defaults or limits.

## Conflict rule

When an agent discovers a required cross-lane design change, it should stop expanding scope, document the issue, propose the smallest contract update, add/adjust an ADR if required, and coordinate before editing another lane's files.
