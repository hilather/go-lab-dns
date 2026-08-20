# Roadmap and Non-goals

Status: Proposed
Owners: Product, Architecture
Last reviewed: 2026-08-19 (Phase 5 operator console 1.1.0 implemented)

## Phase 0: contracts first

- ADRs.
- Canonical domain model.
- Configuration schema.
- DNS semantic fixtures.
- Capability registry.
- Error catalog.
- CI and documentation gates.

## Phase 1: useful DNS service

- UDP/TCP listeners.
- Exact records.
- Authoritative and overlay zones.
- RFC-style wildcard synthesis.
- Common record types.
- Forwarding and cache.
- Explain resolution.
- YAML bootstrap and reset.

## Phase 2: control planes

- REST v1.
- MCP pinned baseline.
- Parity generation and tests.
- Auth, RBAC, audit.
- Plan/apply/export.

## Phase 3: bounded chaos

- Per-record fixed and uniform delay.
- RCODE/NODATA injection.
- UDP drop.
- UDP forced truncation.
- TCP close/reset.
- TTL manipulation.
- Alternate/partial answers.
- Flapping.
- Upstream and cache safe faults.
- Simulation, explanation, expiry, budgets, emergency disable.

## Phase 4: hardening and GA

- Load, race, fuzz, soak, and interoperability testing.
- Container hardening and supply-chain artifacts.
- Deployment repository examples.
- Runbook exercises.
- Compatibility and migration tooling.
- Complete release notes and GA acceptance review.

**1.0.0-rc.1 candidate (2026-08-15):** evidence in [docs/releases/acceptance-evidence.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/acceptance-evidence.md); notes in [docs/releases/v1.0.0-rc.1.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.0.0-rc.1.md). The annotated GA tag is a human step on a green commit. Residual: [docs/known-limitations.md](https://github.com/hilather/go-lab-dns/blob/main/docs/known-limitations.md).

## Phase 5: operator web UI (1.1.0)

**Implemented.** Normative design: [docs/22-web-ui.md](https://github.com/hilather/go-lab-dns/blob/main/docs/22-web-ui.md). ADR: [0008](https://github.com/hilather/go-lab-dns/blob/main/docs/adr/0008-embedded-operator-web-ui.md). Tasks: [tasks/18-web-ui.md](https://github.com/hilather/go-lab-dns/blob/main/tasks/18-web-ui.md). Candidate notes: [docs/releases/v1.1.0.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.1.0.md).

- Embedded React console on the management listener (same origin as REST).
- Session/CSRF for browsers; UI talks REST only.
- Full REST/MCP/UI parity for every public operator capability.
- TanStack Query polling; Playwright matrix; required CI job `web`.

The 1.0.0-rc.1 / rc.2 candidates do **not** include the UI. Those notes were not rewritten.

## Deferred

- Root-hints recursion.
- DNSSEC signing.
- RFC 2136 and zone transfers.
- Multi-replica mutable-state consensus.
- Client-facing DoH/DoQ.
- DHCP integration.
- Arbitrary malformed-wire fuzzing in the main service.
- General network impairment outside DNS.
- Console follow-ons: SSE live updates, OAuth/OIDC, management TLS, shared session store.

Deferred work requires a new task plan and, where architectural, an ADR.
