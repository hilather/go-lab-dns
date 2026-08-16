# Product Acceptance Criteria

Status: Accepted (1.0.0-rc.1 evidence attached)
Owners: Product, Architecture, QA
Last reviewed: 2026-08-15 (GA-001)

Evidence index for this candidate: [docs/releases/acceptance-evidence.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/acceptance-evidence.md). Residual: [docs/known-limitations.md](https://github.com/hilather/go-lab-dns/blob/main/docs/known-limitations.md). Tag is a human step on a green commit.

## Functional DNS

- Exact A, AAAA, CNAME, TXT, MX, SRV, PTR, CAA, NS, SOA, SVCB, and HTTPS behavior passes tests.
- Wildcards follow closest-encloser behavior and exact names win.
- Authoritative NXDOMAIN and NODATA are distinguishable and include SOA.
- Overlay zones fall through only when no local answer exists.
- UDP and TCP both work and are semantically equivalent absent transport-specific chaos.
- Forwarding suffix selection and upstream failover behave as documented.
- Positive and negative caching obey bounds.

## Chaos

- A fixed or distributed delay can be attached to a specific exact or wildcard RRset.
- Delay is cancellable and globally bounded.
- Deterministic seeded decisions reproduce across restarts for a pinned algorithm.
- RCODE, NODATA, drop, truncation, TCP close/reset, TTL, alternate answer, partial answer, ordering, flap, cache, upstream, and rate effects pass regression tests.
- Protected names and clients are not affected.
- Emergency disable works during load.
- Simulation never mutates live state or sleeps.

## State

- Startup loads strict YAML.
- Unknown fields fail.
- Mutations use expected revision and idempotency.
- Candidate state is atomically swapped.
- Reset safely reloads bootstrap.
- Runtime drift is visible.
- Canonical export and deployment operations are deterministic.

## REST and MCP

- Every public capability has parity.
- REST contract and MCP conformance tests pass.
- Shared authorization and errors match.
- MCP Streamable HTTP validates Origin and protocol version.
- All mutations support planning and audit.

## Security

- Service is not an open resolver by default.
- Management is isolated and authenticated.
- Container is non-root, read-only, capability-free, and scanned.
- Chaos has separate scopes, expiry, caps, and emergency controls.
- No secret appears in export, logs, or public errors.

## Quality

- Every area has regression tests.
- Race, fuzz smoke, integration, parity, container, documentation, and security CI passes.
- Load and soak targets are met and documented.
- No known critical or high-severity unresolved vulnerability is accepted for GA without explicit governance.
- Documentation matches implementation.

## Release

- Release notes include all functionality differences from the previous tag.
- API, MCP, config, metrics, CLI, defaults, and chaos action diffs are reviewed.
- All CI passes on the tagged commit.
- Any previous CI failure has a documented fix and appropriate hardening.
- Deployment and rollback are tested.
