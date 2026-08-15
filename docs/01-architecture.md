# System Architecture

Status: Proposed
Owners: Architecture, DNS, Control Plane
Last reviewed: 2026-08-15 (STA-001 bootstrap serve)
Related ADRs: 0001, 0002, 0003, 0004, 0005

## Problem statement

Laboratory devices need a predictable DNS service that can override exact names, synthesize wildcard answers, forward unresolved names to selected upstream resolvers, and deliberately introduce controlled DNS faults. The service must be equally controllable through REST and MCP, be easy for agents to inspect and modify safely, and recover to a Git-controlled YAML baseline by restarting or resetting.

## Goals

- Correct authoritative, overlay, wildcard, forwarding, and cache behavior.
- Per-entry and broader chaos behavior with deterministic testing and strict safety limits.
- One shared application model exposed through REST and MCP.
- Immutable, atomic runtime state.
- Ephemeral operation with GitOps-oriented desired state.
- Container-first deployment and strong observability.
- Clear package ownership and agent-ready maintenance rules.

## Non-goals

- Full Internet recursion from root hints.
- General-purpose authoritative hosting for public Internet zones.
- RFC 2136 dynamic update in the initial release.
- AXFR, IXFR, and secondary-server operation.
- DNSSEC signing of local zones in the initial release.
- Multi-replica runtime-state consensus.
- A web administration UI.
- DHCP integration.
- Arbitrary malformed DNS packet generation in the main service.

## Invariants

1. DNS request handling does not depend on REST or MCP availability.
2. Every query sees one complete immutable runtime snapshot.
3. A state mutation is validated and compiled completely before activation.
4. REST and MCP call the same application capabilities.
5. Bootstrap YAML is read-only to the service.
6. Chaos cannot affect management or health endpoints.
7. Global safety caps override policy requests.
8. Unknown configuration fields are errors.
9. The service is not an open recursive resolver by default.
10. Runtime drift is visible and exportable.

## Context diagram

```text
                       Deployment repository
                  desired YAML, probes, image pin
                              |
                              v
                    read-only bootstrap mount
                              |
                    +---------------------+
Lab clients ------> |      LabDNS         | ------> Upstream DNS pools
 UDP/TCP DNS        |                     |          UDP/TCP/DoT
                    |  immutable snapshot |
                    |  resolver + cache   |
                    |  bounded chaos      |
                    +----------+----------+
                               |
                  management network only
                         REST and MCP
                               |
                       humans and agents
```

## Container and process model

The initial deployment is one process in one container:

- DNS listener: UDP and TCP on an unprivileged container port such as 5353.
- REST listener: HTTP on a management-only port.
- MCP listener: `/mcp` on the same management HTTP server or a separately configured management listener.
- Metrics and health endpoints: management-only.
- No writable persistent volume.
- Read-only bootstrap configuration.
- Optional in-memory audit ring; durable audit collection is external through logs or telemetry.

Host port 53 maps to container port 5353. The process runs as a non-root user with all Linux capabilities dropped.

## High-level component model

```text
DNS wire adapters
  -> query admission
  -> snapshot lookup
  -> local resolver
  -> forwarding/cache
  -> chaos evaluation and bounded execution
  -> DNS response writer

REST adapter -----+
                  +-> shared capability registry -> application service
MCP adapter ------+                                 -> state compiler
                                                    -> atomic snapshot store
                                                    -> audit emitter
```

## Recommended Go package boundaries

```text
cmd/labdns                 process entrypoint and CLI wiring
internal/model             canonical domain types
internal/app               commands, queries, plans, authorization hooks
internal/config            YAML decoding, normalization, schema validation
internal/compiler          immutable snapshot compilation
internal/snapshot          active/previous/bootstrap snapshot store
internal/resolver          exact, wildcard, CNAME, negative answer logic
internal/forwarder         policy selection, upstream exchange, health
internal/cache             positive and negative cache
internal/chaos             selectors, deterministic decisions, effects, budgets
internal/dnswire           third-party DNS library adapter
internal/dnsserver         UDP/TCP listeners, admission, timeouts
internal/control/rest      REST transport adapter
internal/control/mcp       MCP transport adapter
internal/capabilities      capability registry and parity metadata
internal/auth              authentication, scopes, actor identity
internal/audit             mutation and security events
internal/observability     metrics, tracing, structured logs
internal/buildinfo         version, commit, protocol compatibility
api                        source schemas and generated contracts
```

Third-party DNS and MCP types must not escape their adapters.

## Snapshot model

A compiled snapshot contains only immutable or internally concurrency-safe structures:

- Canonical normalized source state.
- Zone suffix trie.
- Owner-name existence tree including empty non-terminals.
- Owner and type to RRset indexes.
- Wildcard source indexes.
- Delegation metadata.
- Forwarding-policy suffix trie.
- Upstream pool configuration.
- Chaos policy indexes by record ID, owner, wildcard source, zone, client group, forwarding rule, upstream pool, and global scope.
- Security and safety-policy indexes.
- State revision and generation metadata.

The active snapshot is held by an atomic pointer. A DNS request loads the pointer once and retains that snapshot for the whole request.

## Control-plane mutation flow

```text
request
  -> authenticate and authorize
  -> validate expected revision and idempotency key
  -> apply operations to canonical state copy
  -> normalize
  -> validate full candidate
  -> compile full candidate snapshot
  -> generate deterministic diff and impact summary
  -> if dry-run: return plan
  -> atomically swap active snapshot
  -> emit audit and state-change event
  -> return new revision
```

No mutation changes the live object graph in place.

## DNS query flow

```text
receive packet
  -> admission and parse limits
  -> load one snapshot
  -> classify client and transport
  -> identify local zone and forwarding policy
  -> evaluate pre-resolution chaos
  -> exact/local/wildcard resolution or upstream/cache resolution
  -> evaluate response chaos
  -> enforce response and transport limits
  -> emit bounded telemetry
  -> write or deliberately suppress response
```

The chaos engine receives a structured resolution context and cannot access management handlers or arbitrary process resources.

## DNS wire adapter and listeners

`internal/dnswire` is the only package that may import `github.com/miekg/dns` (pinned at **v1.1.72**). It converts wire bytes to `model.Query` plus a package-local `Request` (ID, opcode, EDNS) and encodes `model.Result` back to octets. Library types never appear in `dnsserver` or later packages.

`internal/dnsserver` binds UDP and TCP, applies admission, calls `Handler.ServeDNS`, and applies the returned `TransportHint`. It does not import snapshot, resolver, forwarder, or chaos.

`internal/dnsquery` implements that handler: one `Store.Load` per query, client-group classification from the compiled `AccessIndex` (an uncompiled zero index still walks spec CIDRs), zone + forwarding selection, `resolver.Resolve`, then `forwarder.Exchange` only when the client may forward. Chaos `Decide` is not called yet.

`labdns serve --config PATH` loads bootstrap YAML, runs `compiler.Compile`, installs the snapshot on `snapshot.Store` (`SetBootstrap` + `Swap`), and binds UDP/TCP. Invalid bootstrap does not bind DNS. Management HTTP is not started in this slice.

Default listen address (first GA, configured later by CFG): `:5353` UDP+TCP.

Default admission and transport limits:

| Limit | Default |
|---|---|
| Max UDP datagram | 4096 octets (oversize dropped) |
| Max TCP message | 65535 octets (oversize closes the connection) |
| Max questions | 1 (0 or more than 1 → FORMERR) |
| Max EDNS UDP size | 4096 (client sizes below 512 raised to 512) |
| Advertised EDNS UDP size | 1232 |
| TCP idle / read / write / max age | 10s / 2s / 2s / 30s |
| Query handler timeout | 2s |
| Max TCP connections / per source IP | 256 / 16 |
| Max in-flight queries | 1024 |
| Max hold-then-close | 1s |

Parse/admission RCODEs: empty or short datagram → drop; malformed with a 12-byte header → FORMERR (question echoed only if unpacked; never a fabricated `. IN A`); QR=1 → drop; opcode ≠ QUERY → NOTIMP; QCLASS ≠ IN or AXFR/IXFR → NOTIMP; EDNS version ≠ 0 → BADVERS (header RCODE 0 + OPT EXTENDED-RCODE 16, OPT VERSION 0).

Transport hints: TCP-only actions on UDP are **drop** (no successful answer). `HintTruncate` on TCP is **send** of the full response (TC is a UDP signal). Unknown hints are **drop**. After `ServeDNS` returns the server owns the `Response`; later `SetHint` fails.

Metrics hooks take only bounded labels (transport, RCODE, action, reason). QNAME and client IP are not recorded.

## Reverse-proxy relationship

A wildcard A or AAAA record can direct many names to one host. An HTTP reverse proxy or ingress on that host must route by Host or SNI to reach different tools. DNS does not normally select an arbitrary application port for browsers.

## Failure modes

- Invalid bootstrap: fail startup before binding DNS unless an explicit safe fallback is configured.
- Invalid runtime mutation: reject without changing the active snapshot.
- Upstream exhaustion: return bounded SERVFAIL and optional Extended DNS Error.
- Chaos budget exhaustion: skip or clamp the chaos action and emit a metric; do not exhaust request workers.
- Control-plane failure: DNS continues using the active snapshot.
- Telemetry exporter failure: do not block DNS; buffer within strict limits or drop telemetry.
- Previous snapshot unavailable: reset still reloads the bootstrap file.

## Security considerations

The service must enforce client-network allowlists, management-plane authentication, per-capability authorization, request size and rate limits, non-root container execution, and protected names. See the security architecture and threat model.

## Observability

Expose bounded metrics for resolution source, RCODE, transport, latency buckets, upstream status, cache results, chaos policy decisions, clamped effects, state generation, mutations, and auth failures. Never use query names or client IPs as unbounded metric labels.

## Testing strategy

Architecture invariants require unit, property, fuzz, race, integration, parity, container, and fault-injection tests. Snapshot swaps and delayed-query cancellation require race and leak testing.

## Compatibility implications

Public REST paths, MCP tool schemas, configuration versions, DNS semantics, and state export formats are compatibility surfaces. Internal package boundaries are not public APIs.

## Open questions

- Whether the first release supports DNS-over-TLS upstreams or adds them in the next minor release.
- Whether optional local Unix-socket management should be included.
- Which OpenAPI 3.1 patch version and schema toolchain are pinned in the implementation repository.
- Whether a previous-snapshot ring larger than one generation is needed for stale-answer experiments.
