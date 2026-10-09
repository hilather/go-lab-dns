# REST API Design

Status: Implemented (API-001)
Owners: REST, Application
Last reviewed: 2026-10-09 (process bind vs host publish; loopback is 127.0.0.0/8, ::1, IPv4-mapped 127/8; ADR 0011 proposed)
Last reviewed: 2026-10-03 (active-chaos resolve/explain output and duration strings)
Last reviewed: 2026-08-19 (serve wires embedded UI handler)
Last reviewed: 2026-09-01 (resolve useCache does not store Fallthrough)
Related ADRs: 0004, [0010](adr/0010-bearer-profile-no-loopback-exception.md)

## Goals

- A versioned, discoverable, machine-readable API.
- OpenAPI source generated or verified from the capability registry.
- Consistent pagination, filtering, errors, revisions, and idempotency.
- No business logic in handlers.

## Base behavior

- Base path: `/v1`.
- JSON request and response bodies unless exporting YAML (`GET /v1/state:export?format=yaml`, default).
- Problem responses use `application/problem+json` with a stable domain error code. Status hints are produced by `capabilities.ProblemFrom`; `internal/control/rest` only serializes and sets `Instance` (`urn:labdns:request:…`).
- Mutations accept `Idempotency-Key` and expected revision in the body, `If-Match`, or `X-LabDNS-Expected-Revision`.
- Request bodies default to a 1 MiB cap; handlers also enforce a request deadline and a concurrent-request admission cap.
- Management listener default address is `:8080` (`rest.DefaultAddr`). The default process bind is every interface.
- OpenAPI 3.1 is generated from the capability registry and `model.Spec` at [api/openapi/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/openapi/v1.json) (`make generate`).
- Authentication (Q-AUTH, [ADR 0010](adr/0010-bearer-profile-no-loopback-exception.md)): under `dev-loopback-unauth`, unauthenticated access is accepted from loopback peers (`127.0.0.0/8`, `::1`, and IPv4-mapped `127/8`). A same-host reverse proxy or SSH tunnel that dials this process from loopback is that peer ([docs/08-security-architecture.md](08-security-architecture.md)). Under `profile: bearer`, every management request (loopback included) needs `Authorization: Bearer` **or** a live `labdns_session` cookie. `Authorization: Bearer` wins over the cookie and ignores CSRF. Health live/ready skip auth so process probes work. `X-Forwarded-For` is not trusted. Shared RBAC lives in `internal/auth` (same decision as MCP). No permissive CORS headers are emitted; a present non-loopback `Origin` is 403 unless allowlisted (`spec.management.allowedOrigins` on the active snapshot).
- Cookie-authenticated non-GET requests must send `X-LabDNS-CSRF`. CSRF is omitted on `POST /v1/session` **only when no live session cookie is present**. A cookie-present `POST /v1/session` without Bearer rotates the existing session Actor and must not call loopback Identify.
- Management responses set `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and `X-Frame-Options: DENY`. CSP is applied on the SPA/HTML branch only.
- `GET`/`HEAD` paths that are not under `/v1` or `/mcp` are a pre-auth SPA branch: never `Identify`, never 401. If `spec.ui.enabled` is false or the UI handler is nil, those paths 404. `GET /v1/*` still authenticates.

## Endpoints

### Health and build

```text
GET /v1/health/live
GET /v1/health/ready
GET /v1/version
GET /v1/capabilities
GET /v1/status
GET /v1/schema/config
GET /v1/docs/dns-semantics
GET /v1/docs/chaos-safety
```

Health live/ready are process-local probes and are not MCP tools. Paths and operation names are frozen in `internal/capabilities` / `api/capabilities/v1.json`.

### State

```text
GET  /v1/state
POST /v1/state:validate
GET  /v1/state:export
POST /v1/state:reset
POST /v1/changes:plan
POST /v1/changes:apply
```

### DNS data

```text
GET  /v1/zones
GET  /v1/zones/{zoneId}
GET  /v1/zones/{zoneId}/records
GET  /v1/zones/{zoneId}/records/{recordId}
POST /v1/resolve
POST /v1/resolve:explain
```

Typed CRUD routes may be added, but they must compile to the same structured change operations as `changes:apply`.

### Forwarding and cache

```text
GET /v1/forwarding/policies
GET /v1/upstream-pools
GET /v1/upstreams/status
GET /v1/cache/status
POST /v1/cache:flush
```

Cache flush is privileged, bounded by selector, and does not change desired state.

### Chaos

```text
GET  /v1/chaos/status
GET  /v1/chaos/policies
GET  /v1/chaos/policies/{policyId}
POST /v1/chaos:simulate
POST /v1/chaos/policies/{id}:activate
POST /v1/chaos/policies/{id}:deactivate
POST /v1/chaos/policies/{id}:expire
POST /v1/chaos:emergency-disable
POST /v1/chaos:emergency-enable
```

Activate/deactivate/expire templates are frozen as `{id}`. GET uses `{policyId}` for the same policy identifier. Adapters must register the catalog spellings so `LookupREST` matches.

### Audit

```text
GET /v1/audit
GET /v1/audit/{eventId}
```

### Session and UI

```text
POST   /v1/session
GET    /v1/session
DELETE /v1/session
GET    /                 (SPA pre-auth; `cmd/labdns` injects `web.Handler()`)
```

`POST /v1/session` with **no cookie header** (or with Bearer) authenticates with Identify (loopback unauth under `dev-loopback-unauth`, or Bearer) and issues cookie `labdns_session` (`HttpOnly`, `SameSite=Lax`, `Path=/`, host-only, `Secure` iff TLS) plus JSON `{csrf, actor}`. Under `profile: bearer`, that POST from loopback without a bearer or cookie is 401 `authentication required`. `Cache-Control: no-store`. A present but unknown/expired cookie without Bearer is 401 and does not Identify. At 256 distinct sessions, new creates fail `rate_limited` with detail `session table full` (retryable); rotation of an existing session does not consume a slot.

Cookie-present `POST /v1/session` **without** Bearer requires CSRF and rotates ID/CSRF for the **same** Actor (`class=ui-session`). Identity switch requires `Authorization: Bearer`.

`GET /v1/session` requires a live cookie (reload recovery). `DELETE /v1/session` requires a live cookie and CSRF; response 204 with `Max-Age=0`.

Actor JSON copies Identify `id`/`role`/`scopes`/`groups` and sets `class` to `ui-session`. A viewer session rotated on loopback without Bearer does not become administrator.

## Resolve request

```json
{
  "name": "alpha.tools.lab.example.net.",
  "type": "A",
  "clientContext": {
    "clientGroup": "test-devices",
    "transport": "udp"
  },
  "options": {
    "useCache": true,
    "applyChaos": false
  }
}
```

Management resolve returns the base answer by default; `options.applyChaos: true` models active chaos effects. `useCache` reads and writes the process DNS cache; overlay `Fallthrough` results are not stored (management resolve never forwards).

Resolve with `applyChaos: true` models active chaos; `POST /v1/resolve:explain` always includes this model and bypasses the cache. `result.explanation.baseAnswers` preserves the base answer RRsets; `baseRcode` records the base response code. `chaosDecisions` reports `policyId`, `outcomeId`, `triggered`, `skipReason`, `digestHex`, and `delay` (a duration string). Empty optional fields may be omitted. Modeling executes no upstream exchange or delay and changes no live chaos counters or state. When resolve uses the cache, only the base result is read or stored. OpenAPI resolution schemas are generated from shared model types. The explanation fields are additive within `/v1`; RR TTLs and modeled decision delays use duration strings in both REST and MCP output.

## Pagination

Use opaque cursors on zone and record lists (`cursor` + `limit`). `GET /v1/audit` accepts `limit` only; the in-memory ring has no cursor until a later slice. Filters are explicit typed fields, not arbitrary query-language expressions in the first release.

## Conditional and idempotent writes

- Require expected revision for desired-state writes.
- Support `Idempotency-Key` with bounded in-memory retention.
- Return `409 Conflict` for revision mismatch or conflicting key reuse.
- Return the current revision and re-plan information.

## Error response

```json
{
  "type": "urn:labdns:error:revision-conflict",
  "title": "State revision conflict",
  "status": 409,
  "code": "revision_conflict",
  "detail": "The active state changed after the plan was created.",
  "instance": "urn:labdns:request:01J...",
  "currentRevision": "sha256:...",
  "expectedRevision": "sha256:...",
  "retryable": true
}
```

## Implementation notes

The adapter is `internal/control/rest`. Routes are compiled from `capabilities.All()` (catalog spellings, including `{id}:activate` / `:expire`). Handlers call `app.Service` only.

| Concern | Behavior |
|---|---|
| Auth | Loopback unauthenticated only under `dev-loopback-unauth`. Under `bearer`, loopback and remote peers need `Authorization: Bearer` **or** a live `labdns_session` cookie (MCP needs the header). Bearer wins. Cookie non-GET requires `X-LabDNS-CSRF`. `auth.Policy` / `Authenticator` is the shared identity hook. Capability + resource scopes are enforced. Health live/ready stay unauthenticated. |
| Pagination | Opaque `cursor` + `limit` query parameters on zone, record, and audit lists. |
| Timeouts | Per-request context deadline (default 30s). `ListenAndServe` also sets read/write/header timeouts. |
| Request ID | `X-Request-ID` is accepted or generated and echoed; problem `instance` is `urn:labdns:request:<id>`. |
| Health | `GET /v1/health/live` is process liveness. `GET /v1/health/ready` is ready when `Status.Ready` is true (runtime revision present and required listeners bound). Upstream failure is `Status.Degraded`, not unready. Chaos must not affect either probe. |

`labdns serve --config PATH` constructs `rest.Server` with `Config.Addr` from YAML (empty → `:8080`) and the process `app.Service`. `--management-listen` overrides the bind address; `off` / `none` / `-` leaves management unbound. Health live/ready stay reachable without a bearer token.

## Security considerations

Validate Origin where browser access is possible, disable permissive CORS by default, authenticate before parsing large bodies where the server stack permits, and rate-limit by actor and source network. First GA trusts `RemoteAddr` only (no forwarded-client spoofing) and emits no `Access-Control-Allow-Origin`.

## Observability

Include request and trace IDs in headers. Metrics use operation IDs from the capability registry. Audit mutation bodies in normalized redacted form, not raw credentials or secrets.

## Testing strategy

- OpenAPI validation.
- Handler/schema contract tests.
- Auth and scope tests.
- Body and timeout limit tests.
- Error mapping tests.
- Parity goldens against MCP.
- Regression tests for every endpoint defect.

## Compatibility implications

Path, method, operation ID, field meaning, default, error code, and status behavior are versioned. Additive optional fields are preferred within `/v1`.
