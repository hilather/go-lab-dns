# Program Board

Status: Proposed
Last reviewed: 2026-08-15

## Work packages

| Order | Task | ID | Depends on | Primary output | Status |
|---:|---|---|---|---|---|
| 1 | Repository foundation | FND-001 | None | Go module, CI skeleton, Make targets | done |
| 2 | Domain and configuration model | CFG-001 | FND-001 | Canonical model, strict YAML, schema | done |
| 3 | DNS wire server | DNS-001 | FND-001, core model interfaces from CFG-001 | UDP/TCP listeners and adapter | done |
| 4 | Local resolver and wildcards | RES-001 | CFG-001, DNS-001 | Exact, wildcard, authoritative, overlay | done |
| 5 | Forwarding and cache | FWD-001 | CFG-001, DNS-001, RES-001 interfaces | Upstream pools and cache | not-started |
| 6 | Snapshot state and mutations | STA-001 | CFG-001, resolver compile interfaces | Atomic state, plan/apply/reset/export | in-progress (app.Service mutations; REST/MCP later) |
| 7 | Chaos core | CHA-001 | CFG-001, RES-001, STA-001 | Policy compiler, selectors, budgets | done |
| 8 | Chaos effects | CHA-002 | CHA-001, FWD-001 | Delay, faults, TTL, answers, transport | done |
| 9 | REST control plane | API-001 | STA-001, capability registry contract | REST v1 and OpenAPI | not-started |
| 10 | MCP control plane | MCP-001 | STA-001, capability registry contract | MCP tools/resources and parity | done |
| 11 | Auth, security, and audit | SEC-001 | API-001/MCP-001 interfaces, DNS admission interfaces | RBAC, limits, audit | not-started |
| 12 | Observability | OBS-001 | Stable component interfaces | Metrics, logs, health, tracing | complete |
| 11 | Auth, security, and audit | SEC-001 | API-001/MCP-001 interfaces, DNS admission interfaces | RBAC, limits, audit | implemented |
| 12 | Observability | OBS-001 | Stable component interfaces | Metrics, logs, health, tracing | not-started |
| 13 | CLI and container | DEP-001 | Runnable server | CLI, image, graceful lifecycle | not-started |
| 12 | Observability | OBS-001 | Stable component interfaces | Metrics, logs, health, tracing | not-started |
| 13 | CLI and container | DEP-001 | Runnable server | CLI, image, graceful lifecycle | done |
| 14 | Deployment examples | GIT-001 | DEP-001, schemas, probes | Compose/Kubernetes/GitOps guidance | not-started |
| 15 | CI, docs, and release automation | REL-001 | FND-001, stable generation targets | Required CI and release diffs | done |
| 16 | Performance and interoperability | PERF-001 | Feature-complete candidate | Load, soak, external compatibility | not-started |
| 17 | GA hardening | GA-001 | All prior tasks | Acceptance review and release candidate | not-started |

## Milestones

### M0: Contracts compile

- FND-001 and CFG-001 complete.
- ADRs accepted.
- Schema and semantic test fixtures exist.
- CI runs formatting, lint, unit, generated, and docs checks.

### M1: DNS usable without control plane

- DNS-001, RES-001, FWD-001, and basic snapshot startup complete.
- Exact, wildcard, authoritative, overlay, cache, and forwarding probes pass over UDP and TCP.

### M2: Agent-controllable

- STA-001, API-001, MCP-001, and parity tests complete.
- Plan/apply/export/reset work through both transports.

### M3: Chaos-capable and secured

- CHA-001, CHA-002, SEC-001, and OBS-001 complete.
- Per-entry delay and all initial safe effects pass regression tests.
- Emergency disable works under load.

### M4: Deployable release candidate

- DEP-001, GIT-001, REL-001, PERF-001 complete.
- Documentation is current.
- Release differences are generated and curated.

### M5: GA

- GA-001 acceptance review passes.
- All required CI passes on the tag commit.
- Complete release notes are approved.

## Cross-cutting blockers

The coordinator must stop dependent work when any of these are unstable:

- Canonical IDs and names.
- Configuration schema source.
- Capability registry API.
- Domain error shape.
- Snapshot compile contract.
- Chaos action phase model.
- Supported MCP protocol version.
