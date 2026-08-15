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

Go constructors and the closed catalog live in `internal/domainerr`. Do not invent synonyms.

Stable codes include:

```text
validation_failed
revision_conflict
idempotency_conflict
not_found
method_not_allowed
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

REST uses `application/problem+json`, appropriate status codes, and the domain error in extension fields. Helpers live in `internal/capabilities` (`ProblemFrom`); `internal/control/rest` serializes that document and does not re-implement the table.

`type` is `urn:labdns:error:` plus the code with underscores replaced by hyphens (example: `urn:labdns:error:revision-conflict`).

| Domain code | HTTP status |
|---|---|
| `validation_failed` | 400 |
| `unsupported_protocol_version` | 400 |
| `unauthenticated` | 401 |
| `forbidden` | 403 |
| `protected_object` | 403 |
| `not_found` | 404 |
| `method_not_allowed` | 405 |
| `revision_conflict` | 409 |
| `idempotency_conflict` | 409 |
| `already_exists` | 409 |
| `chaos_disabled` | 409 |
| `policy_expired` | 409 |
| `rate_limited` | 429 |
| `chaos_budget_exceeded` | 429 |
| `unsupported_capability` | 501 |
| `resolution_failed` | 502 |
| `upstream_unavailable` | 503 |
| `internal_error` | 500 |

Unknown or non-domain errors map to `internal_error` / 500 and must not leak raw text.

## MCP mapping

MCP uses JSON-RPC errors with the same domain error under `data`. Tool-level expected failures may use structured tool error results only when consistent with the pinned MCP SDK and specification; the domain code remains stable. Helpers live in `internal/capabilities` (`JSONRPCFrom`).

JSON-RPC transport codes differ from HTTP; `data.code` matches REST.

| Domain code | JSON-RPC code |
|---|---|
| `validation_failed` | -32602 |
| `unsupported_capability` | -32601 |
| `method_not_allowed` | -32601 |
| `unsupported_protocol_version` | -32600 |
| `unauthenticated` | -32001 |
| `forbidden` | -32003 |
| `protected_object` | -32003 |
| `not_found` | -32004 |
| `rate_limited` | -32005 |
| `chaos_budget_exceeded` | -32005 |
| `revision_conflict` | -32009 |
| `idempotency_conflict` | -32009 |
| `already_exists` | -32009 |
| `chaos_disabled` | -32009 |
| `policy_expired` | -32009 |
| `upstream_unavailable` | -32013 |
| `resolution_failed` | -32013 |
| `internal_error` | -32603 |

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
