# MCP API Design

Status: Proposed
Owners: MCP, Application
Last reviewed: 2026-08-15
Target protocol baseline: 2026-07-28
Related ADRs: 0004, 0006

## Problem statement

Agents need a first-class control interface that is schema-rich, safe, stateless, explainable, and equivalent to REST. MCP changes over time, so the implementation must pin and test protocol versions instead of following an unversioned moving target.

## Goals

- Implement the official MCP protocol through the official Go SDK behind an adapter.
- Expose all public control capabilities with REST parity.
- Use stateless, per-request metadata behavior for the pinned protocol baseline.
- Provide small, typed tools rather than a generic command executor.
- Return structured content and stable domain errors.

## Non-goals

- Generic shell execution.
- Arbitrary file or network access.
- Hidden agent-only mutations.
- Depending on connection identity for state or authorization.

## Transport

Primary transport:

- Streamable HTTP at `/mcp` on the management listener.
- One POST endpoint.
- Request-scoped JSON or SSE responses as required by the pinned protocol.
- Origin validation.
- Authentication and authorization shared with REST.
- Explicit protocol-version negotiation/validation according to the pinned MCP version.

Optional developer transport:

- Stdio adapter exposing the same registry.
- Logs go to stderr, never stdout.
- Credentials come from environment or process-launch context.

## Statelessness

Do not rely on connection-scoped initialization, client identity, selected zone, or previous tool calls. Every tool input and request metadata contains the information needed to process the operation. Long-lived or multi-step application state uses explicit plan IDs or revisions passed by the caller.

## Tools

Recommended names:

```text
dns_version_get
dns_capabilities_get
dns_status_get
dns_schema_get
dns_docs_get

dns_state_get
dns_state_validate
dns_change_plan
dns_change_apply
dns_state_export
dns_state_reset

dns_zones_list
dns_zone_get
dns_records_list
dns_record_get

dns_resolve
dns_explain_resolution

dns_forwarding_policies_list
dns_upstream_pools_list
dns_upstreams_status
dns_cache_status
dns_cache_flush

dns_chaos_status
dns_chaos_policies_list
dns_chaos_policy_get
dns_chaos_simulate
dns_chaos_activate
dns_chaos_deactivate
dns_chaos_set_expiry
dns_chaos_emergency_disable
dns_chaos_emergency_enable

dns_audit_query
dns_audit_get
```

These names are frozen in `internal/capabilities`. Health live/ready have no tools. `dns_docs_get` is parameterized (`id=dns-semantics` or `id=chaos-safety`).

Tools use explicit nouns and verbs, stable schemas, and descriptions that state whether the operation is read-only, state-changing, reversible, or high-impact.

## Resources

Useful read-only resources:

```text
labdns://state
labdns://capabilities
labdns://schema/config
labdns://zones/{zoneId}
labdns://records/{recordId}
labdns://chaos/policies/{policyId}
labdns://upstreams
labdns://audit/recent
labdns://docs/dns-semantics
labdns://docs/chaos-safety
```

A resource mirrors a REST representation or versioned static documentation. Resource authorization is the same as the equivalent GET capability.

## Prompts

Optional prompts may guide a user or agent through safe workflows, such as:

- Plan a DNS override.
- Diagnose why a name resolved a certain way.
- Design a bounded chaos experiment.
- Convert runtime drift into a deployment-repository change.

Prompts do not introduce new capabilities and do not bypass parity or authorization.

## Mutation workflow

Agents should normally:

1. Read current state and revision.
2. Call a typed plan tool.
3. Review normalized diff, affected names, wildcard coverage, clients, chaos maximums, expiry, and probes.
4. Obtain human approval when required by the host.
5. Apply with expected revision and idempotency key.
6. Run resolve/explain probes.
7. Export deployment-repository operations.

## Structured results

Each tool returns machine-readable structured content. Human-readable summaries are secondary and must not be the only representation.

Example chaos simulation result:

```json
{
  "stateRevision": "sha256:...",
  "query": {
    "name": "alpha.tools.lab.example.net.",
    "type": "A",
    "clientGroup": "test-devices",
    "transport": "udp"
  },
  "matchedPolicies": [
    {
      "id": "slow-tools",
      "selectedOutcome": "very-slow",
      "actions": [
        {"type": "delay", "effectiveDuration": "2s"}
      ],
      "clamped": false
    }
  ],
  "finalBehavior": "respond-after-delay"
}
```

## Domain errors

Map shared domain errors to MCP JSON-RPC errors with stable `data.code`, `data.retryable`, revision fields, field violations, and remediation hints. Do not encode failures only as unstructured text. `capabilities.JSONRPCFrom` is the shared helper; adapters must not invent a second mapping.

## Security considerations

- Tool descriptions are not authorization.
- The server validates every request independent of client-provided annotations.
- High-impact tools require dedicated scopes.
- The MCP endpoint validates Origin and protects against DNS rebinding.
- Exposed tools do not allow arbitrary paths, commands, URLs, or packet bytes.
- MCP request metadata and self-reported client identity are not trusted for security decisions unless bound by the authentication layer.

## Observability

Record tool name, protocol version, result, latency, auth result, and request correlation. Do not log full sensitive inputs. Use OpenTelemetry and stderr as appropriate rather than depending on deprecated protocol logging features.

## Testing strategy

- Official SDK conformance tests where available.
- Pinned protocol-version tests.
- Streamable HTTP and stdio tests.
- Origin validation tests.
- Schema and structured-content tests.
- REST/MCP parity goldens.
- Authorization and cancellation tests.
- Backward-compatibility tests for any explicitly supported older protocol version.

## Compatibility implications

MCP protocol versions, tool names, schemas, resource URIs, and result/error structures are public surfaces. The implementation records supported protocol versions in build metadata and the capability resource.

## Open questions

- MCP protocol versions: first GA pins **2026-07-28 only** (ADR 0006).
- Stdio: developer adapter only; not in the production image.
