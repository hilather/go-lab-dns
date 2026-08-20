# Program Board

Status: done (1.1.0 operator console; 1.0.0-rc.1/rc.2 tags remain UI-less; tag-gate pending)
Last reviewed: 2026-08-19 (UI-004 / M6; cleaned duplicate SEC/OBS/DEP/GIT/REL rows)

## Work packages

| Order | Task | ID | Depends on | Primary output | Status |
|---:|---|---|---|---|---|
| 1 | Repository foundation | FND-001 | None | Go module, CI skeleton, Make targets | done |
| 2 | Domain and configuration model | CFG-001 | FND-001 | Canonical model, strict YAML, schema | done |
| 3 | DNS wire server | DNS-001 | FND-001, core model interfaces from CFG-001 | UDP/TCP listeners and adapter | done |
| 4 | Local resolver and wildcards | RES-001 | CFG-001, DNS-001 | Exact, wildcard, authoritative, overlay | done |
| 5 | Forwarding and cache | FWD-001 | CFG-001, DNS-001, RES-001 interfaces | Upstream pools and cache | done |
| 6 | Snapshot state and mutations | STA-001 | CFG-001, resolver compile interfaces | Atomic state, plan/apply/reset/export | done |
| 7 | Chaos core | CHA-001 | CFG-001, RES-001, STA-001 | Policy compiler, selectors, budgets | done |
| 8 | Chaos effects | CHA-002 | CHA-001, FWD-001 | Delay, faults, TTL, answers, transport | done |
| 9 | REST control plane | API-001 | STA-001, capability registry contract | REST v1 and OpenAPI | done |
| 10 | MCP control plane | MCP-001 | STA-001, capability registry contract | MCP tools/resources and parity | done |
| 11 | Auth, security, and audit | SEC-001 | API-001/MCP-001 interfaces, DNS admission interfaces | RBAC, limits, audit | done |
| 12 | Observability | OBS-001 | Stable component interfaces | Metrics, logs, health, tracing | done |
| 13 | CLI and container | DEP-001 | Runnable server | CLI, image, graceful lifecycle | done |
| 14 | Deployment examples | GIT-001 | DEP-001, schemas, probes | Compose/Kubernetes/GitOps guidance | done |
| 15 | CI, docs, and release automation | REL-001 | FND-001, stable generation targets | Required CI and release diffs | done |
| 16 | Performance and interoperability | PERF-001 | Feature-complete candidate | Load, soak, external compatibility | done |
| 17 | GA hardening | GA-001 | All prior tasks | Acceptance review and release candidate | done |
| 18 | Operator web UI foundation | UI-001 | API-001, MCP-001, SEC-001 | Session, embed, login, CI `web` | done |
| 19 | Operator web UI reads | UI-002 | UI-001 | Read pages, generated client, polling | done |
| 20 | Operator web UI mutations | UI-003 | UI-001 | Plan/apply, reset, flush, chaos | done |
| 21 | Operator web UI ship | UI-004 | UI-002, UI-003 | Playwright, a11y, docs, 1.1.0 notes | done |

Duplicate historical SEC-001 / OBS-001 / DEP-001 / GIT-001 / REL-001 rows with conflicting statuses were removed. The unique row for each ID is **done**.

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

- GA-001 acceptance review passes. Evidence: [docs/releases/acceptance-evidence.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/acceptance-evidence.md). Candidate notes: [docs/releases/v1.0.0-rc.1.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.0.0-rc.1.md).
- All required CI must pass on the **tag** commit. This candidate does not create a tag; a human tags the exact green SHA.
- Complete release notes are approved for 1.0.0-rc.1. A later `v1.0.0` tag needs its own notes file.

### M6: Operator web UI (1.1.0)

- UI-001 through UI-004 complete ([tasks/18-web-ui.md](18-web-ui.md)).
- Every `PARITY_REQUIRED` capability is completable in the embedded console.
- `make web-test`, `make web-build`, `make web-e2e`, and CI job `web` are required.
- Notes: do not rewrite rc.1/rc.2; ship as 1.1.0. Candidate: [docs/releases/v1.1.0.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.1.0.md).

## Cross-cutting blockers

The coordinator must stop dependent work when any of these are unstable:

- Canonical IDs and names.
- Configuration schema source.
- Capability registry API.
- Domain error shape.
- Snapshot compile contract.
- Chaos action phase model.
- Supported MCP protocol version.
