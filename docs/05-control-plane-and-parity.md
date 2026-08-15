# Control Plane and REST/MCP Parity

Status: Proposed normative behavior
Owners: Application, REST, MCP
Last reviewed: 2026-08-15 (capability registry freeze)
Related ADRs: 0004, 0006

## Problem statement

Two independently implemented control planes would drift, authorize differently, return different errors, and create unsafe agent behavior. REST and MCP must be two protocol adapters over one capability model.

## Goals

- Semantic parity between REST and MCP.
- Shared schemas, domain types, authorization, validation, planning, apply, and audit behavior.
- Agent-friendly introspection and structured errors.
- Deterministic mutation and conflict handling.

## Non-goals

- Mapping every HTTP transport detail directly to MCP.
- Letting MCP prompts create hidden capabilities.
- Transport-specific business logic.

## Capability registry

Each public capability is declared once:

```go
type Capability struct {
    Name             string
    Version          string
    Description      string
    InputSchema      SchemaRef
    OutputSchema     SchemaRef
    RequiredScopes   []string
    Mutating         bool
    Idempotent       bool
    Handler          Handler
    REST             *RESTBinding
    MCP              *MCPBinding
}
```

The registry is the source for:

- REST route registration.
- OpenAPI operation metadata.
- MCP tool or resource registration.
- Scope documentation.
- Parity tests.
- Capability manifests embedded in releases.

## Parity rules

- Every public REST write operation has one or more MCP tools with equivalent semantics.
- Every MCP mutation tool has a REST operation.
- REST GET representations may map to MCP resources or read tools.
- Status codes and JSON-RPC codes differ by transport, but domain error codes and error data match.
- Pagination, filtering, revisions, and authorization semantics match.
- Default values are applied in the shared application layer.
- Audit records identify the original transport but otherwise use the same event schema.

## Core capabilities

Frozen names live in `internal/capabilities` and the generated manifest `api/capabilities/v1.json`. Renaming a tool, resource, or REST path requires a coordinated catalog + manifest + design-table change. Health live/ready are REST-only process probes and are not MCP tools.

| Capability | REST | MCP tool / resource | Scopes |
|---|---|---|---|
| Health live | `GET /v1/health/live` | *not a tool* (process-local) | none |
| Health ready | `GET /v1/health/ready` | *not a tool* | none |
| Version | `GET /v1/version` | `dns_version_get` | `dns.read` |
| Capabilities | `GET /v1/capabilities` | `dns_capabilities_get`, `labdns://capabilities` | `dns.read` |
| Agent status | `GET /v1/status` | `dns_status_get`, `labdns://status` | `dns.read` |
| Config schema | `GET /v1/schema/config` | `dns_schema_get`, `labdns://schema/config` | `dns.read` |
| Get state | `GET /v1/state` | `dns_state_get`, `labdns://state` | `dns.read` |
| Validate | `POST /v1/state:validate` | `dns_state_validate` | `dns.write` |
| Plan | `POST /v1/changes:plan` | `dns_change_plan` | `dns.write` |
| Apply | `POST /v1/changes:apply` | `dns_change_apply` | `dns.write` |
| Export | `GET /v1/state:export` | `dns_state_export` | `dns.read` |
| Reset | `POST /v1/state:reset` | `dns_state_reset` | `dns.admin` |
| Zones list/get | `GET /v1/zones`, `GET /v1/zones/{zoneId}` | `dns_zones_list`, `dns_zone_get`, `labdns://zones/{zoneId}` | `dns.read` |
| Records list/get | `GET /v1/zones/{zoneId}/records`, `GET /v1/zones/{zoneId}/records/{recordId}` | `dns_records_list`, `dns_record_get`, `labdns://records/{recordId}` | `dns.read` |
| Resolve | `POST /v1/resolve` | `dns_resolve` | `dns.read` |
| Explain | `POST /v1/resolve:explain` | `dns_explain_resolution` | `dns.read` |
| Forwarding | `GET /v1/forwarding/policies` | `dns_forwarding_policies_list` | `dns.forwarders.read` |
| Pools | `GET /v1/upstream-pools` | `dns_upstream_pools_list` | `dns.forwarders.read` |
| Upstream status | `GET /v1/upstreams/status` | `dns_upstreams_status`, `labdns://upstreams` | `dns.forwarders.read` |
| Cache status | `GET /v1/cache/status` | `dns_cache_status` | `dns.read` |
| Cache flush | `POST /v1/cache:flush` | `dns_cache_flush` | `dns.admin` |
| Chaos status | `GET /v1/chaos/status` | `dns_chaos_status` | `dns.chaos.read` |
| Chaos policies | `GET /v1/chaos/policies`, `GET /v1/chaos/policies/{policyId}` | `dns_chaos_policies_list`, `dns_chaos_policy_get`, `labdns://chaos/policies/{policyId}` | `dns.chaos.read` |
| Simulate | `POST /v1/chaos:simulate` | `dns_chaos_simulate` | `dns.chaos.read` |
| Activate / deactivate | `POST /v1/chaos/policies/{id}:activate`, `:deactivate` | `dns_chaos_activate`, `dns_chaos_deactivate` | `dns.chaos.activate` |
| Set expiry | `POST /v1/chaos/policies/{id}:expire` | `dns_chaos_set_expiry` | `dns.chaos.activate` |
| Emergency disable / enable | `POST /v1/chaos:emergency-disable`, `:emergency-enable` | `dns_chaos_emergency_disable`, `dns_chaos_emergency_enable` | `dns.chaos.emergency` |
| Audit list | `GET /v1/audit` | `dns_audit_query`, `labdns://audit/recent` | `dns.audit.read` |
| Audit get | `GET /v1/audit/{eventId}` | `dns_audit_get` | `dns.audit.read` |
| Docs: DNS semantics | `GET /v1/docs/dns-semantics` | `dns_docs_get` (`id=dns-semantics`), `labdns://docs/dns-semantics` | `dns.read` |
| Docs: chaos safety | `GET /v1/docs/chaos-safety` | `dns_docs_get` (`id=chaos-safety`), `labdns://docs/chaos-safety` | `dns.read` |

The HTTP-less handler surface is `internal/app.Service`. REST and MCP must call it; they must not reimplement plan/apply/reset. Chaos activate/deactivate/simulate/set-expiry currently return `unsupported_capability` until CHA-001.

`internal/capabilities` maps `domainerr` to REST `application/problem+json` status hints and MCP JSON-RPC `data` without implementing either server.

## Mutation contract

All mutations accept common metadata:

- Expected revision.
- Idempotency key.
- Reason.
- Optional change ticket.
- Dry-run or apply mode.
- Operations or typed desired object.

All mutations return:

- Previous and candidate revision.
- Normalized diff.
- Validation warnings and errors.
- Impact summary.
- Authorization decision metadata safe for the caller.
- Audit event ID when applied.
- Deployment-repository operations.

## Agent-first impact summary

A plan should include:

- Names and zones that may change.
- Whether wildcard coverage changes.
- Whether authoritative misses change.
- Client groups affected.
- Upstream and forwarding changes.
- Chaos policies activated, their expiry, and maximum modeled effect.
- Compatibility warnings.
- Required permissions.
- Suggested verification probes.

## Authorization

Authorization is capability- and resource-aware. Example scopes:

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

The application handler receives an authenticated actor context from the adapter and makes one shared authorization decision.

## Failure modes

- Adapter schema mismatch: parity CI fails and blocks merge.
- Revision conflict: stable domain code `revision_conflict`.
- Unsupported capability version: return explicit supported versions.
- MCP client protocol mismatch: reject before invoking application logic.
- Partial apply: forbidden by architecture; if commit fails, active revision remains unchanged.

## Security considerations

MCP tools are powerful and model-controlled. Mutation descriptions, scopes, dry-run results, expiry, and human approval integration must be clear. Origin validation and authorization are mandatory for MCP over HTTP.

## Observability

Measure calls by capability, transport, result, latency, actor class, and authorization outcome. Avoid high-cardinality actor IDs in metrics; use audit logs for exact identity.

## Testing strategy

- Registry completeness tests.
- Generated OpenAPI and MCP manifest comparison.
- Cross-transport golden input/output tests.
- Shared authorization tests.
- Error mapping tests.
- Idempotency and revision conflict tests.
- Release-time schema diff reports.

## Compatibility implications

Capability names and schema versions are stable public surfaces. Renaming an MCP tool or changing REST semantics requires compatibility treatment even when the shared handler remains unchanged.

## Open questions

- Read tools and resources: first GA ships both (implementation design / ADR 0004). Resources mirror REST GET representations; every list/get also has a read tool so clients without resource support stay unblocked.
