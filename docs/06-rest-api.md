# REST API Design

Status: Proposed
Owners: REST, Application
Last reviewed: 2026-08-15
Related ADRs: 0004

## Goals

- A versioned, discoverable, machine-readable API.
- OpenAPI source generated or verified from the capability registry.
- Consistent pagination, filtering, errors, revisions, and idempotency.
- No business logic in handlers.

## Base behavior

- Base path: `/v1`.
- JSON request and response bodies unless exporting YAML.
- Problem responses use `application/problem+json` with a stable domain error code. Status hints are produced by `capabilities.ProblemFrom` (no HTTP server in the registry package).
- Mutations accept `Idempotency-Key` and expected revision in the body or a documented conditional header.
- Request bodies and response sizes are bounded.
- Management listener is not public by default.

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

The normal management resolve operation defaults to not consuming live chaos decisions. An explicit authorized option can model or apply chaos.

## Pagination

Use opaque cursors. Stable ordering must be documented. Filters are explicit typed fields, not arbitrary query-language expressions in the first release.

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

## Security considerations

Validate Origin where browser access is possible, disable permissive CORS by default, authenticate before parsing large bodies where the server stack permits, and rate-limit by actor and source network.

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
