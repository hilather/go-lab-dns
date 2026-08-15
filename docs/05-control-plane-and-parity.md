# Control Plane and REST/MCP Parity

Status: Proposed normative behavior
Owners: Application, REST, MCP
Last reviewed: 2026-08-15 (STA-001 app.Service mutation contract)
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

| Capability | REST | MCP |
|---|---|---|
| Get state | `GET /v1/state` | resource `labdns://state` or `dns_state_get` |
| Validate candidate | `POST /v1/state:validate` | `dns_state_validate` |
| Plan changes | `POST /v1/changes:plan` | `dns_change_plan` |
| Apply changes | `POST /v1/changes:apply` | `dns_change_apply` |
| Export state | `GET /v1/state:export` | `dns_state_export` |
| Reset bootstrap | `POST /v1/state:reset` | `dns_state_reset` |
| List/get zones | `GET /v1/zones` | `dns_zones_list`, resources |
| Resolve | `POST /v1/resolve` | `dns_resolve` |
| Explain | `POST /v1/resolve:explain` | `dns_explain_resolution` |
| Upstream status | `GET /v1/upstreams/status` | resource or `dns_upstreams_status` |
| List/get chaos | `GET /v1/chaos/policies` | `dns_chaos_policies_list` |
| Simulate chaos | `POST /v1/chaos:simulate` | `dns_chaos_simulate` |
| Activate chaos | `POST /v1/chaos/policies/{id}:activate` | `dns_chaos_activate` |
| Deactivate chaos | `POST /v1/chaos/policies/{id}:deactivate` | `dns_chaos_deactivate` |
| Emergency disable | `POST /v1/chaos:emergency-disable` | `dns_chaos_emergency_disable` |
| Query audit | `GET /v1/audit` | resource or `dns_audit_query` |

The HTTP-less handler surface is `internal/app.Service`. REST and MCP must call it; they must not reimplement plan/apply/reset. Chaos activate/deactivate/simulate/set-expiry currently return `unsupported_capability` until CHA-001.

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

- Whether read-only MCP resources are sufficient for all list/get capabilities or read tools are also provided for clients with limited resource support.
