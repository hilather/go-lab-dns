# State and Configuration

Status: Proposed normative behavior
Owners: Configuration, Application
Last reviewed: 2026-08-15
Related ADRs: 0003, 0005

Canonical Go types live in `internal/model`. YAML decode, default materialization, and JSON Schema are a later configuration PR.

## Problem statement

The service must start from a human-reviewable YAML file, accept temporary runtime changes, expose drift, reset cleanly, and avoid hidden durable state. Agents need strong schemas, deterministic diffs, and safe concurrency.

## Goals

- Strict, versioned YAML desired state.
- Immutable compiled runtime snapshots.
- Ephemeral runtime mutation with revision checks.
- Deterministic canonical export.
- Safe reset-to-bootstrap.
- Deployment-repository patch generation.

## Non-goals

- Internal database persistence.
- Direct Git commits from the DNS process.
- Multi-node state consensus.

## State layers

1. Bootstrap desired state: read-only YAML mounted into the container.
2. Canonical runtime source state: normalized in-memory representation.
3. Compiled runtime snapshot: optimized immutable structures used by queries.
4. Previous snapshot: optional single-generation rollback aid.
5. External durable state: deployment repository outside this project.

## Startup

```text
read file
 -> reject unknown fields
 -> decode versioned schema
 -> normalize names, durations, defaults, and IDs
 -> validate cross-references and policy constraints
 -> compile snapshot
 -> compute bootstrap and runtime revisions
 -> bind listeners
```

A normal startup does not bind DNS when bootstrap validation fails. An explicit emergency mode may start a management-only process for inspection but must not silently serve an empty resolver.

## Revisions

Use a content hash over the canonical normalized state, not raw YAML formatting. Expose:

```json
{
  "bootstrapRevision": "sha256:...",
  "runtimeRevision": "sha256:...",
  "generation": 18,
  "drifted": true,
  "loadedAt": "2026-08-15T20:00:00Z"
}
```

Generation is process-local and monotonically increasing. Revision is content-addressed and portable.

## Mutation envelope

```json
{
  "expectedRevision": "sha256:...",
  "idempotencyKey": "01J...",
  "reason": "Test resolver timeout handling",
  "operations": [],
  "mode": "plan-or-apply"
}
```

Requirements:

- `expectedRevision` is required for writes except an explicitly privileged bootstrap reset.
- Idempotency keys are retained in a bounded in-memory cache.
- Repeated keys with the same request return the original result.
- Repeated keys with different requests return a conflict.
- Plans and applies use the same validation and compilation path.
- The returned diff is based on canonical state.

## Reset

Reset rereads the mounted bootstrap file, validates and compiles it, and swaps only after success. A bad replacement file leaves the current runtime state active. Reset clears runtime idempotency entries and optional runtime-only policy activation metadata as documented.

## Export

Export provides:

- Canonical YAML.
- Canonical JSON.
- Current revision.
- Bootstrap-to-runtime structured operations.
- Human-readable diff.
- Deployment-repository patch guidance.

The service does not write the mounted file.

## Configuration outline

```yaml
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: primary-lab

spec:
  listeners: {}
  access: {}
  defaults: {}
  zones: []
  forwarding: {}
  cache: {}
  chaos: {}
  observability: {}
  management: {}
```

## Illustrative configuration

```yaml
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: primary-lab

spec:
  listeners:
    dns:
      address: ":5353"
      protocols: [udp, tcp]
    management:
      address: ":8080"
      restPath: /v1
      mcpPath: /mcp

  access:
    clientGroups:
      - id: test-devices
        cidrs: [10.42.0.0/16]
      - id: management
        cidrs: [10.42.255.0/24]
        chaosExempt: true

  defaults:
    ttl: 30s
    negativeTTL: 10s

  zones:
    - id: lab-zone
      name: lab.example.net.
      mode: authoritative
      soa:
        primary: ns1.lab.example.net.
        administrator: hostmaster.lab.example.net.
        serial: auto
        refresh: 1h
        retry: 5m
        expire: 24h
      nameservers: [ns1.lab.example.net.]
      records:
        - id: ns1-a
          owner: ns1
          type: A
          values: [10.42.0.53]
        - id: tools-wildcard-a
          owner: "*.tools"
          type: A
          ttl: 30s
          values: [10.42.0.20]
          chaosPolicyRefs: [slow-tools]
        - id: grafana-cname
          owner: grafana.tools
          type: CNAME
          values: [gateway.lab.example.net.]

    - id: vendor-overlay
      name: vendor.example.
      mode: overlay
      records:
        - id: vendor-special-a
          owner: special-api
          type: A
          values: [10.42.0.30]

  forwarding:
    policies:
      - id: corp-policy
        suffix: corp.example.net.
        upstreamPool: corporate
      - id: default-policy
        suffix: .
        upstreamPool: default
    pools:
      - id: corporate
        strategy: ordered
        upstreams:
          - id: corp-1
            endpoint: 10.0.0.53:53
            transport: udp
      - id: default
        strategy: health-aware
        upstreams:
          - id: default-1
            endpoint: 10.0.0.54:53
            transport: udp
          - id: default-2
            endpoint: 10.0.0.55:53
            transport: tcp

  cache:
    enabled: true
    maxEntries: 10000
    minimumTTL: 1s
    maximumTTL: 5m
    maximumNegativeTTL: 1m

  chaos:
    enabled: true
    safety:
      protectedNames: [dns.lab.example.net.]
      protectedClientGroups: [management]
      maxDelay: 10s
      maxConcurrentDelayed: 2000
      maxDropProbability: 0.5
    policies:
      - id: slow-tools
        owner: platform-lab
        reason: Test application startup timeouts
        enabled: false
        safetyClass: low
        scope:
          recordIds: [tools-wildcard-a]
          clientGroups: [test-devices]
        selector:
          mode: deterministic
          seed: startup-v1
          probability: 1.0
        outcomes:
          - id: delayed
            weight: 100
            actions:
              - type: delay
                phase: before-response
                distribution: uniform
                min: 100ms
                max: 750ms
```

## Frozen v1alpha1 field names

JSON object names match the YAML keys in the sample above. Additional frozen fields used by the Go types:

| Area | Fields / values |
|---|---|
| Document | `apiVersion: labdns.dev/v1alpha1`, `kind: LabDNS`, `metadata.name`, `metadata.labels` |
| Access | `unknownClient` only `refuse-forward`; `clientGroups[].id`, `cidrs`, `chaosExempt`, `allowForward` (materialized default **true** when a group exists; Go zero value is false → local only, RA=0) |
| Defaults | `ttl`, `negativeTTL`, `cnameDepth` (safe default **8**) |
| Zone | `id`, `name`, `mode` (`authoritative` \| `overlay`), `soa`, `nameservers`, `records` |
| Record | `id`, `owner`, `type`, `ttl`, `values`, `genericRdata`, `chaosPolicyRefs` |
| First-GA types | `A`, `AAAA`, `CNAME`, `TXT`, `MX`, `SRV`, `PTR`, `CAA`, `NS`, `SOA`, `SVCB`, `HTTPS`, plus validated generic RDATA (`typeCode`, `presentation`) |
| Forwarding | `policies[]` (`id`, `suffix`, `upstreamPool`, `failover`), `pools[]` (`id`, `strategy`, `upstreams`) |
| Pool strategy | `ordered` \| `round-robin` \| `random` \| `health-aware` |
| Upstream | `id`, `endpoint`, `transport` (`udp` \| `tcp` only) |
| Failover | `timeout`, `onTimeout`, `onTransportError`, `onSERVFAIL`, `onREFUSED`, `udpTruncateRetryTCP` |
| Cache | `enabled`, `maxEntries`, `minimumTTL`, `maximumTTL`, `maximumNegativeTTL`, `staleServing` |
| Chaos | `enabled`, `emergencyDisabled`, `safety`, `policies` per [docs/03-chaos-engine.md](https://github.com/hilather/go-lab-dns/blob/main/docs/03-chaos-engine.md) (`id`, `owner`, `reason`, `enabled`, `expiresAt`, `safetyClass`, `scope`, `selector`, `outcomes`, `composition`) |
| Management | `auth.profile` (`dev-loopback-unauth` \| `bearer`), `auth.secretRef` |
| Observability | `logQNAME` |

IDs (`zone`, `record`, `forwardingPolicy`, `upstream`, `upstreamPool`, `clientGroup`, `chaosPolicy`) are **user-supplied and required**. The server does not generate them.

## Operations

Typed change sets use:

```json
{"op":"add|update|remove","target":{"kind":"...","id":"...","zoneId":"..."},"value":{}}
```

`target.kind` values: `zone`, `record`, `forwardingPolicy`, `upstreamPool`, `upstream`, `clientGroup`, `chaosPolicy`, `chaosSafety`, `cache`, `defaults`, `listeners`, `access`, `observability`, `management`, `chaosActivation`. `zoneId` is required when `kind` is `record`. Replace-entire-object is `update` on the singleton targets. Activate/deactivate/set-expiry is `update` + `chaosActivation`. There is no JSON Patch profile.

## Schema rules

- Unknown fields fail validation.
- Stable IDs are required for zones, records, forwarding policies, upstreams, client groups, and chaos policies.
- IDs are user-supplied and immutable within an API version.
- Names are canonicalized during normalization.
- Durations use an explicit documented syntax.
- Cross-references must resolve.
- Duplicate semantic RRsets are merged only when explicitly allowed; otherwise reject ambiguity.
- Defaults are materialized in canonical export.
- Secrets are references, never inline plaintext fields intended for Git.

## Failure modes

- Revision conflict: return conflict with current revision and a re-plan hint.
- Compile failure: return structured field and invariant errors; active snapshot unchanged.
- Idempotency cache full: evict by bounded policy and expose metrics.
- Bootstrap file unavailable at reset: reject reset; active snapshot unchanged.

## Security considerations

Configuration can redirect traffic and activate faults. Validate addresses, names, target suffixes, scopes, and maximums. Authorization may restrict which zones or client groups an actor may change.

## Observability

Expose generation, revisions, drift, validation failures by stable error code, mutation latency, compile latency, and reset outcomes. Do not include secrets or entire configuration in logs.

## Testing strategy

Use schema tests, unknown-field tests, normalization goldens, canonical round trips, revision stability tests, cross-reference tests, mutation conflict tests, reset failure tests, and fuzzing of YAML/JSON decoders.

## Compatibility implications

Config API versions are explicit. Removing or reinterpreting a field requires a new version or a documented migration. New optional fields must have safe defaults.

## Open questions

Resolved for first GA:

- Stable IDs are user-supplied only.
- Canonical export does not preserve comments (no sidecar).
