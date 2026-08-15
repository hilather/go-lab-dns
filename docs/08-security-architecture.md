# Security Architecture

Status: Proposed
Owners: Security, DNS, Control Plane
Last reviewed: 2026-08-15
Related ADRs: 0003, 0004, 0005, 0007

## Goals

- Prevent operation as an open resolver or amplification service.
- Prevent unauthorized traffic redirection and chaos activation.
- Limit damage from malformed input, overload, and upstream failure.
- Protect credentials, audit data, and management interfaces.
- Keep secure behavior independent of agent behavior.

## Trust boundaries

1. Lab DNS clients to DNS listener.
2. Management clients and agents to REST/MCP listener.
3. LabDNS to upstream resolvers.
4. Container to host/kernel.
5. Runtime process to read-only bootstrap file.
6. Telemetry exporter to external observability systems.
7. Deployment pipeline to image registry and deployment host.

## DNS-plane controls

- Client CIDR allowlists or authenticated network boundary.
- Separate recursion availability by client group.
- Query rate and concurrency limits.
- Maximum question count, packet size, EDNS size, CNAME depth, answer count, and upstream attempts.
- UDP response size controls and minimal responses.
- TCP read/write/idle/total deadlines and per-source connection caps.

First-GA DNS listener numeric defaults (DNS-001; YAML overrides land with CFG/STA): max UDP 4096, max TCP message 65535, one question, EDNS UDP clamp 4096 / advertise 1232, TCP idle 10s, read/write 2s, connection max-age 30s, query timeout 2s, 256 TCP connections (16 per source IP), 1024 in-flight queries, hold-then-close cap 1s. Oversized UDP is dropped; oversized TCP is closed. Rate limits beyond these caps are SEC-001.
- Self-forwarding and loop validation.
- No automatic use of host `/etc/resolv.conf` unless explicitly configured and documented.

## Management-plane controls

- Bind to loopback or dedicated management network by default.
- TLS for remote access.
- Workload identity, mTLS, OAuth-compatible bearer tokens, or reverse-proxy identity as a deployment choice.
- Shared auth middleware for REST and MCP.
- Resource-aware RBAC.
- Origin validation for MCP Streamable HTTP and browser-reachable REST.
- Strict body, header, rate, and timeout limits.
- No permissive CORS by default.

## Chaos privilege separation

Suggested roles:

- Viewer: inspect state and explain resolution.
- DNS editor: edit zones and records, not forwarders or chaos.
- Forwarder operator: edit upstream policies.
- Chaos designer: create disabled policies.
- Chaos operator: activate low/medium policies within limits.
- Chaos admin: activate high-impact policies and emergency-enable.
- Emergency operator: disable all chaos.
- Administrator: reset state and manage protected policy.

Creation and activation are separate capabilities so a policy can be reviewed before it becomes live.

## Protected objects

Deployment policy defines:

- Protected names and zones.
- Protected record IDs.
- Protected client groups.
- Management and monitoring networks.
- Maximum address ranges for alternate answers.
- Upstream endpoints that ordinary roles cannot change.
- Emergency controls that cannot be removed through runtime mutation.

## Secret management

Bootstrap configuration contains secret references, not secret values. The process may receive credentials through mounted secret files, environment variables, workload identity, or a local agent. Secret material is excluded from state export, diffs, logs, MCP resources, and audit payloads.

## Audit

Audit every mutation, activation, deactivation, reset, emergency action, rejected authorization, and security-policy change. Record:

- Event ID and time.
- Authenticated actor and credential class.
- Transport and capability.
- Reason and change reference.
- Previous and new revision.
- Normalized redacted diff.
- Result and stable error code.

Audit delivery failure cannot block DNS but may block high-impact management writes if deployment policy requires durable audit confirmation.

## Supply chain

- Pin direct dependencies.
- Generate an SBOM.
- Scan source, dependencies, and images.
- Use reproducible or provenance-attested builds where practical.
- Sign release tags and container images where supported.
- Run as a non-root UID in a minimal image.
- Use read-only root filesystem, tmpfs for temporary files, dropped capabilities, and no-new-privileges.

## Chaos abuse prevention

- Global caps are not mutable by ordinary chaos roles.
- High-impact runtime policies require expiry.
- Alternate answers are allowlisted.
- Drop and delay are capped.
- Management plane is out of scope for the chaos engine.
- Unsafe malformed-wire actions are absent.
- Emergency disable is tested in every release.

## Failure modes

- Auth provider unavailable: fail closed for writes; read behavior follows documented policy.
- Audit sink unavailable: use bounded buffering; high-impact writes may fail closed by policy.
- Time source unreliable: reject new absolute-schedule policies when clock health is unknown; durations may use monotonic time.
- TLS certificate expired: local emergency access remains available according to deployment design.

## Observability

Security metrics include denied DNS clients, rate-limit events, management auth failures, scope denials, protected-object violations, high-impact policy count, emergency-disable state, and audit delivery failures. Avoid sensitive labels.

## Testing strategy

- Auth and RBAC matrix tests.
- Network allowlist tests.
- Origin and DNS rebinding defense tests.
- Request limit and rate-limit tests.
- Protected object and alternate-address tests.
- Container hardening tests.
- Dependency and image scans.
- Threat-model regression tests.

## Compatibility implications

Weakening a default or broadening access is a security-significant breaking change even if schemas remain compatible.

## Open questions

- Default remote authentication profile for reference deployments.
- Whether durable audit acknowledgment is required for high-impact chaos activation.
