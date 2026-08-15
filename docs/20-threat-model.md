# Threat Model

Status: Proposed
Owners: Security
Last reviewed: 2026-08-15

## Assets

- Integrity of DNS answers.
- Availability and latency of DNS service.
- Management credentials and authorization policy.
- Bootstrap and runtime state.
- Audit integrity.
- Upstream resolver credentials and endpoints.
- Laboratory network privacy.

## Threat actors

- Untrusted device on an adjacent network.
- Compromised laboratory client.
- Over-privileged or compromised agent credential.
- Malicious or faulty upstream resolver.
- Supply-chain attacker.
- Accidental operator or agent error.

## Threats and mitigations

### Open resolver and amplification

Mitigate with client allowlists, recursion policy, rate limits, response-size controls, and network isolation.

### Unauthorized traffic redirection

Mitigate with authenticated management, resource-aware scopes, planning, revision checks, audit, protected zones, and reviewed deployment state.

### Chaos used as denial of service

Mitigate with separate activation privileges, global immutable caps, expiry, protected clients/names, concurrency budgets, cancellation, and emergency disable.

### Agent tool misuse

Mitigate with typed tools, no shell/file/network primitives, shared authorization, dry-run, impact summaries, human approval support, and idempotency.

### DNS rebinding against management MCP

Mitigate with management network isolation, Origin validation, Host validation where appropriate, TLS, and authentication.

### Parser exploitation

Mitigate with mature libraries behind adapters, strict limits, fuzzing, dependency updates, and process sandboxing.

### Upstream loop or exfiltration

Mitigate with explicit endpoints, suffix policies, self-loop detection, allowlists, TLS where required, and no silent host-resolver fallback.

### State race or partial update

Mitigate with immutable snapshots, full candidate compile, atomic swap, expected revisions, and race tests.

### Secret leakage

Mitigate with secret references, redaction, no raw config logs, bounded audit payloads, and export filtering.

### Supply-chain compromise

Mitigate with pinned dependencies, review, SBOM, scans, signed tags/images, provenance, minimal runtime image, and reproducible-build goals.

### Telemetry denial or cardinality explosion

Mitigate with bounded queues, label allowlists, no QNAME labels, sampling, and exporter isolation.

## Abuse cases to test

- High-rate randomized names.
- Oversized EDNS requests.
- TCP connection exhaustion.
- CNAME loops.
- Long labels and malformed compression.
- Repeated revision-conflict attempts.
- Idempotency-key memory pressure.
- Maximum-delay policy under peak load.
- Attempts to target protected names or management clients.
- Alternate-answer attempts outside allowed CIDRs.
- MCP requests with invalid Origin or protocol version.

## Residual risk

A properly authorized DNS or chaos administrator can intentionally disrupt lab name resolution. Governance, approval, audit, expiry, and deployment isolation reduce but do not eliminate that inherent authority.
