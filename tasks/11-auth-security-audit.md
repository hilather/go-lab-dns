# SEC-001: Authentication, Authorization, Security Limits, and Audit

Status: not-started
Recommended owner: Security agent
Dependencies: DNS admission, REST/MCP adapter hooks, application capabilities
Exclusive ownership: `internal/auth`, `internal/audit`, security policy enforcement

## Goal

Enforce secure DNS and management defaults, shared RBAC, protected objects, abuse limits, and auditable state changes.

## Work items

- [ ] Implement configurable management authentication adapters behind one identity interface.
- [ ] Implement capability- and resource-aware scopes.
- [ ] Separate chaos design, activation, high-impact activation, and emergency privileges.
- [ ] Implement DNS client-network allowlists and recursion availability by client group.
- [ ] Implement management request rate/concurrency limits.
- [ ] Implement DNS query, UDP response, TCP connection, and upstream concurrency limits.
- [ ] Implement protected zones, records, names, clients, upstreams, and immutable safety caps.
- [ ] Implement secret references and redaction.
- [ ] Implement normalized mutation and security audit events.
- [ ] Implement bounded audit buffering and delivery policy.
- [ ] Add security headers and CORS/Origin policies.
- [ ] Add container and dependency security checks with DEP-001/REL-001.

## Required tests

- [ ] Full role/scope/capability matrix.
- [ ] REST/MCP shared authorization equivalence.
- [ ] DNS allowlist and open-resolver prevention tests.
- [ ] Rate, concurrency, packet, answer, and connection limit tests.
- [ ] Protected-object mutation and chaos tests.
- [ ] Alternate-answer and upstream allowlist tests.
- [ ] Secret redaction tests for logs, export, errors, and audit.
- [ ] Origin and CORS tests.
- [ ] Audit event and delivery-failure tests.
- [ ] Threat-model abuse cases.
- [ ] Regression test for every security defect.

## Documentation updates

- [ ] Publish reference auth profiles and scope catalog.
- [ ] Update threat model and security architecture.
- [ ] Document audit retention/delivery assumptions.
- [ ] Add security release-note entries.

## Acceptance criteria

- Default deployment is not an open resolver and does not expose unauthenticated management remotely.
- Chaos privileges are separate and protected controls cannot be changed by ordinary roles.
- Security tests and scans pass.
- No known secret leakage path remains in tested surfaces.

## Handoff

Provide policy configuration, scope matrix, audit schema, and deployment requirements.
