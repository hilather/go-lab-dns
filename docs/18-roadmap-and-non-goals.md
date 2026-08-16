# Roadmap and Non-goals

Status: Proposed
Owners: Product, Architecture
Last reviewed: 2026-08-15 (GA-001 1.0.0-rc.1 candidate)

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

## Deferred

- Root-hints recursion.
- DNSSEC signing.
- RFC 2136 and zone transfers.
- Multi-replica mutable-state consensus.
- Client-facing DoH/DoQ.
- Web UI.
- DHCP integration.
- Arbitrary malformed-wire fuzzing in the main service.
- General network impairment outside DNS.

Deferred work requires a new task plan and, where architectural, an ADR.
