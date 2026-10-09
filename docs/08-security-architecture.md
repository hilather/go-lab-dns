# Security Architecture

Status: Implemented (SEC-001)
Owners: Security, DNS, Control Plane
Last reviewed: 2026-10-09 (startup warning when dev-loopback-unauth binds management beyond loopback; ADR 0011 proposed)
Last reviewed: 2026-10-09 (nil authenticator fails closed; an unreported profile loses only the loopback exception)
Last reviewed: 2026-10-08 (bearer profile has no loopback exception; ADR 0010)
Last reviewed: 2026-10-03 (browser cookie mutation races, fail-closed recovery, upstream reply correlation and cancellation)
Last reviewed: 2026-10-03 (safe management mounts and current caller protections on cached resolve)
Last reviewed: 2026-10-03 (sequential authorization of atomic change sets)
Last reviewed: 2026-08-31 (protected-name wildcard synthesis)
Last reviewed: 2026-10-03 (upstream reply correlation and cancellation)
Related ADRs: 0003, 0004, 0005, 0007, [0010](adr/0010-bearer-profile-no-loopback-exception.md), [0011](adr/0011-propose-bearer-default-profile.md)

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
- Self-forwarding and loop validation.
- Upstream reply ID, QR, opcode, and full question correlation; incomplete truncated replies fail closed when a complete TCP retry is unavailable. Cancellation closes upstream sockets to release resources promptly.
- No automatic use of host `/etc/resolv.conf` unless explicitly configured and documented.

### First-GA DNS listener defaults

First-GA DNS listener numeric defaults (DNS-001; YAML overrides land with CFG/STA): max UDP 4096, max TCP message 65535, one question, EDNS UDP clamp 4096 / advertise 1232, TCP idle 10s, read/write 2s, connection max-age 30s, query timeout 2s, 256 TCP connections (16 per source IP), 1024 in-flight queries, hold-then-close cap 1s. Oversized UDP is dropped; oversized TCP is closed. Per-source query token buckets (default 256/s, burst 512) sit on top of those caps. Refuse-forward for unknown clients shipped with CFG/FWD and is not re-specified here.

## Management-plane controls

- Bind to loopback or dedicated management network by default.
- TLS for remote access.
- Workload identity, mTLS, OAuth-compatible bearer tokens, or reverse-proxy identity as a deployment choice.
- Shared auth middleware for REST and MCP.
- Resource-aware RBAC.
- Origin validation for MCP Streamable HTTP and browser-reachable REST. A present non-loopback Origin is rejected unless it is on the adapter allowlist (DNS-rebinding default-deny). Missing Origin is allowed for official SDK/curl clients.
- Strict body, header, rate, and timeout limits. Management default is 32 requests/s per source (burst 64) plus a 256-wide concurrency gate.
- No permissive CORS by default: OPTIONS is 403 and no `Access-Control-Allow-*` headers are emitted.

### First-GA auth profiles

| Profile | Loopback (`127.0.0.0/8`, `::1`, IPv4-mapped `127/8`) | Non-loopback |
|---|---|---|
| `dev-loopback-unauth` (default) | Unauthenticated, treated as administrator | Bearer token required |
| `bearer` | No exception: `Authorization: Bearer` or a live REST `labdns_session` cookie (MCP needs the header; [ADR 0010](adr/0010-bearer-profile-no-loopback-exception.md)) | Bearer token required; `secretRef` must resolve to at least one token |

`bearer` tokens are loaded from `spec.management.auth.secretRef` (a file: one token, or JSON `{"tokens":[{"token","id","role","scopes"}]}`). Unknown tokens fail closed. Health live/ready stay unauthenticated in both profiles. MCP stdio (`LocalOrStdio`) has no network peer and is unchanged. `X-Forwarded-For` is not trusted.

`dev-loopback-unauth` treats the TCP peer as administrator when that peer is loopback (`127.0.0.0/8`, `::1`, or IPv4-mapped loopback). `X-Forwarded-For` is not a peer. The default management address is `:8080`, which listens on every interface. A same-host reverse proxy or SSH tunnel that dials this process from loopback is the peer, so every client behind it is administrator. That remains true when the socket is bound only to loopback: the proxy is local. It is also true of the default all-interfaces bind, which accepts that loopback dial and every other interface. Publishing the port on the host loopback (Compose `127.0.0.1:8080:8080`) does not change the address the process bound inside its network namespace; `examples/labdns-deploy` still sets the in-container address to `:8080`. A client that reaches the process through that publish is not a loopback peer: docker-proxy dials the container from the bridge network, so under `dev-loopback-unauth` it gets 401 like any other non-loopback peer (verified for `examples/compose.smoke.yaml`: host `POST /v1/session` and `GET /v1/state` return 401, the health probes 200). `labdns serve` prints one stdout warning after the listening line when this profile is active and the bound management address is not loopback. The warning quotes that bound address (after `--management-listen`, and after `Listen` resolves `localhost` or rewrites `0.0.0.0`). It is not printed for `--management-listen=off` (also `none`, `-`, `unbound`), for a loopback bind (`127.0.0.1`, another `127.0.0.0/8` address, `::1`, or IPv4-mapped loopback, including `localhost` when it resolves there), or for `profile: bearer`. Reset and apply do not rebind the listener or reload the authenticator, so they do not print it again. Two separate controls apply. (1) A loopback bind (`127.0.0.1` or `[::1]`) limits which interfaces can reach the listener; it does not remove the administrator path, because a same-host reverse proxy or SSH tunnel still dials from loopback. (2) `profile: bearer` removes the unauthenticated administrator path for every peer, loopback included ([ADR 0010](adr/0010-bearer-profile-no-loopback-exception.md)). Use `bearer` whenever a proxy or tunnel fronts management. Changing the omitted-profile default to `bearer` is proposed and is not this release ([ADR 0011](adr/0011-propose-bearer-default-profile.md)).

A nil management authenticator fails closed: every non-probe request is unauthenticated (`authentication required`), including loopback and including a presented bearer. An authenticator that does not report a profile, or that reports an empty or unknown profile, is not the `dev-loopback-unauth` exception. Only an explicit `dev-loopback-unauth` report keeps loopback-without-a-bearer as administrator; a presented bearer is still checked by that authenticator. An omitted YAML profile still normalizes to `dev-loopback-unauth` (`config.Normalize`, `NewPolicy`), so configured deployments are unchanged. `labdns serve` passes the loaded `*auth.Policy` whenever management is bound, and does not construct REST or MCP when there is no canonical state to load a profile from.

### Browser session and CSRF

Browser session POST and DELETE operations are serialized within each page. A superseded successful login is revoked using only its response CSRF token before another cookie mutation can run. Session recovery waits for this queue. Failed revocation or logout retains its CSRF token and blocks recovery until cleanup succeeds; an explicit bearer login can replace the session and clear that pending state. A successful cookie response with an unreadable or invalid session body also blocks recovery until explicit bearer sign-in or confirmed logout. This ordering is page-local and does not coordinate other tabs.

The operator console authenticates with an in-process session table (max 256, 12h sliding TTL) and cookie `labdns_session` (`HttpOnly`, `SameSite=Lax`, `Path=/`, host-only, `Secure` iff `r.TLS != nil`). CSRF secret is returned in JSON and required as `X-LabDNS-CSRF` on cookie-authenticated non-GET requests (`subtle.ConstantTimeCompare`). CSRF is omitted on `POST /v1/session` **only when no session cookie is sent**. A live-cookie POST without Bearer **rotates** ID/CSRF for the existing Actor (`class=ui-session`) and must not call Identify (under `dev-loopback-unauth`, loopback Identify would escalate a viewer to administrator; under `bearer`, it would 401). A present but unknown/expired cookie without Bearer is 401 (SPA clears it after GET `/v1/session` 401); first login omits the cookie. Identity switch requires `Authorization: Bearer`. `Authorization: Bearer` wins over cookie and CSRF for that request.

Session create copies Identify `id`/`role`/`scopes`/`groups`. `ClassUISession` plus `administrator` role still yields all scopes via role expansion. MCP ignores cookies: cookie-only MCP is 401 for every peer under `bearer`, and off-loopback under `dev-loopback-unauth`. Cookie value, CSRF, and bearer are never logged. Cap reject uses existing `rate_limited` (429, detail `session table full`); do not evict.

GET/HEAD outside `/v1` and `/mcp` is a pre-auth SPA branch and must not 401. Management JSON still gets nosniff / frame-deny / referrer-policy; CSP is applied on HTML/SPA.

Browser identity transitions cancel and clear cached queries, including previous actors' scopes, audit entries, and state. Generation checks prevent late session responses or delayed body parsing from restoring a previous actor or replacing a newer CSRF secret. Sign-out clears the in-memory CSRF immediately, including when the revoke request fails.

### Scope catalog

Frozen spellings (adapters must not invent synonyms):

```text
dns.read
dns.write
dns.admin
dns.forwarders.read
dns.forwarders.write
dns.chaos.read
dns.chaos.write
dns.chaos.activate
dns.chaos.emergency
dns.audit.read
```

`dns.admin` satisfies every scope. Plan/apply/validate are resource-aware: the change set, not the catalog `dns.write` row, decides which write family is required.

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

| Role | Scopes |
|---|---|
| Viewer | `dns.read`, `dns.forwarders.read`, `dns.chaos.read` |
| DNS editor | `dns.read`, `dns.write` |
| Forwarder operator | `dns.read`, `dns.forwarders.read`, `dns.forwarders.write` |
| Chaos designer | `dns.read`, `dns.chaos.read`, `dns.chaos.write` (disabled policies only) |
| Chaos operator | `dns.read`, `dns.chaos.read`, `dns.chaos.activate` (low/medium) |
| Chaos admin | chaos read/write/activate/emergency (high-impact activate + emergency enable) |
| Emergency operator | `dns.read`, `dns.chaos.read`, `dns.chaos.emergency` (disable only) |
| Administrator | all scopes, including protected-object and safety-cap mutation |

High-impact (`safetyClass: high`) activation requires chaos-admin (activate **and** emergency) or administrator. Emergency enable uses the same split so an emergency-only operator cannot re-enable.

## Protected objects

Deployment policy defines:

- Protected names and zones.
- Protected record IDs.
- Protected client groups.
- Management and monitoring networks.
- Maximum address ranges for alternate answers.
- Upstream endpoints that ordinary roles cannot change.
- Emergency controls that cannot be removed through runtime mutation.

Plan/apply/validate expand relative record owners against the zone origin before the protected-name check, matching desired-state canonicalization. A wildcard owner (`*.<parent>`) is denied for ordinary roles when a protected name sits under that parent — closest-encloser synthesis would otherwise answer the protected QNAME without an exact owner. A zone update or remove is denied for ordinary roles when the current zone is protected or already contains a protected record — zone replace is whole-object, so omitting that record would delete it. Administrators may still mutate protected objects.

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

Audit delivery failure cannot block DNS. First GA does **not** fail-close management writes on an external sink (Q-AUDIT): the in-memory ring is the process-local record; an optional hook is best-effort and its errors are counted. Retention is the ring bound (default 128 events, newest kept).

## Supply chain

- Pin direct dependencies.
- Pin GitHub Actions by commit SHA (with a version comment); the toolchain version lives in one `GO_VERSION` env var per workflow, kept in sync with `go.mod` and the `Dockerfile`.
- Generate an SBOM.
- Scan source, dependencies, and images.
- Use reproducible or provenance-attested builds where practical.
- Sign release tags and container images where supported.
- Run as a non-root UID in a minimal image (`ghcr.io/hilather/labdns`, UID 65532, no shell).
- Use read-only root filesystem, tmpfs for temporary files, dropped capabilities, and no-new-privileges.

## Chaos abuse prevention

- Global caps are not mutable by ordinary chaos roles.
- High-impact runtime policies require expiry.
- Alternate answers are allowlisted.
- Drop and delay are capped.
- Management plane is out of scope for the chaos engine.
- Unsafe malformed-wire actions are absent.
- Emergency disable is tested in every release.

Management mounts reject REST and operator-console route collisions and ServeMux pattern syntax. Cached management results reconstruct the current query context before chaos modeling, so protected clients and exempt groups cannot inherit another caller’s selectors or explanation context.

## Failure modes

- Auth provider unavailable: fail closed for writes; read behavior follows documented policy.
- Audit sink unavailable: use bounded buffering; high-impact writes may fail closed by policy.
- Time source unreliable: reject new absolute-schedule policies when clock health is unknown; durations may use monotonic time.
- TLS certificate expired: local emergency access remains available according to deployment design.

## Observability

Security metrics include denied DNS clients, rate-limit events, management auth failures, scope denials, protected-object violations, high-impact policy count, emergency-disable state, and audit delivery failures. Avoid sensitive labels.

## Testing strategy

- Auth and RBAC matrix tests, including a nil authenticator on every non-probe request and a missing profile that loses only the loopback exception.
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

Atomic change sets authorize each operation against the private candidate produced by preceding operations, in addition to checking the starting state. A policy created or upgraded earlier in a batch retains its actual safety class for activation checks; relative owners in newly added zones use that zone origin. Denied batches leave the active snapshot unchanged. Required permission metadata includes the scopes required at each intermediate step.

Authorization decodes only fields relevant to the security decision (policy activation and safety class, zone names, record IDs and owners). Duration strings are validated by the shared mutation decoder; valid `ttl: "30s"`, selector time buckets, and delay durations do not become authorization failures. Unknown fields and malformed duration values still fail candidate validation.
