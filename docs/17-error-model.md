# Error Model

Status: Proposed
Owners: Application, DNS, REST, MCP
Last reviewed: 2026-08-15

## Goals

- Stable machine-readable errors shared by REST and MCP.
- Clear DNS RCODE and EDE behavior.
- Safe remediation hints for agents.
- No secrets or stack traces in public responses.

## Domain error shape

```json
{
  "code": "validation_failed",
  "message": "Candidate state is invalid.",
  "retryable": false,
  "fieldViolations": [
    {
      "path": "spec.chaos.policies[0].outcomes[0]",
      "code": "conflicting_transport_actions",
      "message": "drop and tcp-reset cannot be selected in one outcome"
    }
  ],
  "currentRevision": "sha256:...",
  "remediation": "Split the transport actions into exclusive outcomes."
}
```

Stable codes include:

```text
validation_failed
revision_conflict
idempotency_conflict
not_found
already_exists
forbidden
unauthenticated
rate_limited
protected_object
chaos_disabled
chaos_budget_exceeded
policy_expired
unsupported_capability
unsupported_protocol_version
upstream_unavailable
resolution_failed
internal_error
```

## REST mapping

REST uses `application/problem+json`, appropriate status codes, and the domain error in extension fields.

## MCP mapping

MCP uses JSON-RPC errors with the same domain error under `data`. Tool-level expected failures may use structured tool error results only when consistent with the pinned MCP SDK and specification; the domain code remains stable.

## DNS mapping

DNS clients receive DNS-standard outcomes, not internal JSON errors. Examples:

- Malformed request: FORMERR.
- Unsupported operation: NOTIMP.
- Unauthorized recursion: REFUSED, optionally EDE Prohibited.
- Upstream or internal bounded failure: SERVFAIL with suitable optional EDE.
- Authoritative nonexistence: NXDOMAIN or NODATA with SOA.
- Deliberate chaos: selected syntactically correct RCODE/EDE or transport behavior.

Internal audit and explanation records include the domain code that caused the DNS result.

## Retry guidance

`retryable` is advisory and based on error class, not a promise. Revision conflict is retryable after re-read and re-plan. Validation errors are not retryable without input changes. Rate limits include a bounded retry hint when safe.

## Testing strategy

- Cross-transport mapping goldens.
- No-secret and no-stack-trace tests.
- Field-path tests.
- DNS RCODE/EDE packet tests.
- Compatibility diff of the error code catalog.
