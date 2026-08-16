# SEC-001: Authentication, Authorization, Security Limits, and Audit

Status: done
Recommended owner: Security agent
Dependencies: DNS admission, REST/MCP adapter hooks, application capabilities
Exclusive ownership: `internal/auth`, `internal/audit`, security policy enforcement

## Goal

Enforce secure DNS and management defaults, shared RBAC, protected objects, abuse limits, and auditable state changes.

## Work items

- [x] Implement configurable management authentication adapters behind one identity interface.
- [x] Implement capability- and resource-aware scopes.
- [x] Separate chaos design, activation, high-impact activation, and emergency privileges.
- [x] Implement DNS client-network allowlists and recursion availability by client group.
- [x] Implement management request rate/concurrency limits.
- [x] Implement DNS query, UDP response, TCP connection, and upstream concurrency limits.
- [x] Implement protected zones, records, names, clients, upstreams, and immutable safety caps.
- [x] Implement secret references and redaction.
- [x] Implement normalized mutation and security audit events.
- [x] Implement bounded audit buffering and delivery policy.
- [x] Add security headers and CORS/Origin policies.
- [ ] Add container and dependency security checks with DEP-001/REL-001.

## Required tests

- [x] Full role/scope/capability matrix.
- [x] REST/MCP shared authorization equivalence.
- [x] DNS allowlist and open-resolver prevention tests.
- [x] Rate, concurrency, packet, answer, and connection limit tests.
- [x] Protected-object mutation and chaos tests.
- [x] Alternate-answer and upstream allowlist tests.
- [x] Secret redaction tests for logs, export, errors, and audit.
- [x] Origin and CORS tests.
- [x] Audit event and delivery-failure tests.
- [x] Threat-model abuse cases.
- [ ] Regression test for every security defect.

## Documentation updates

- [x] Publish reference auth profiles and scope catalog.
- [x] Update threat model and security architecture.
- [x] Document audit retention/delivery assumptions.
- [x] Add security release-note entries.

## Acceptance criteria

- Default deployment is not an open resolver and does not expose unauthenticated management remotely.
- Chaos privileges are separate and protected controls cannot be changed by ordinary roles.
- Security tests and scans pass.
- No known secret leakage path remains in tested surfaces.

## Handoff

Policy configuration: `spec.management.auth.profile` (`dev-loopback-unauth` | `bearer`) and `secretRef`. Scope matrix and roles: [docs/08-security-architecture.md](https://github.com/hilather/go-lab-dns/blob/main/docs/08-security-architecture.md). Audit schema: `internal/audit.Event` (ring default 128; hook is best-effort, Q-AUDIT no fail-closed). Deployment still binds management to loopback or a dedicated network (DEP-001 / GIT-001).
