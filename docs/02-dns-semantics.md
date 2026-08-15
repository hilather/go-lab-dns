# DNS Semantics

Status: Implementation-confirmed for local resolution (RES-001)
Owners: DNS
Last reviewed: 2026-08-15
Related ADRs: 0002, 0005

## Problem statement

The product needs predictable behavior when local records, wildcard records, authoritative zones, overlay zones, cache entries, forwarding rules, and chaos policies overlap. Ambiguity would make agents unsafe and tests unreliable.

## Goals

- Define deterministic query processing.
- Follow standard wildcard closest-encloser behavior.
- Distinguish authoritative misses from overlay fallthrough.
- Preserve DNS transport and flag correctness.
- Produce explainable, testable outcomes.

## Non-goals

- General recursive resolution.
- DNSSEC signing or validation policy beyond safe pass-through behavior.
- Dynamic update and zone transfer semantics.

## Canonical names

- Internally store fully qualified, lower-case ASCII DNS names with a trailing dot.
- Preserve presentation data where required but compare canonical wire-equivalent names.
- Apply IDNA processing only through one reviewed adapter and document the selected profile.
- Reject names that exceed DNS wire limits.
- Give every configured RRset a stable record ID independent of owner text.

## Zone modes

### Authoritative

A matching authoritative zone owns its namespace. A missing owner returns NXDOMAIN. An existing owner without the requested type returns NODATA. These misses do not forward.

### Overlay

A matching overlay zone returns a local answer when a local exact or wildcard rule **resolves the requested type** (or a CNAME that can be followed). If no local rule resolves it — including empty non-terminals, existing owners with a different type, and wildcard sources that do not have the requested type or CNAME — processing continues to forwarding (`Fallthrough=true`). Overlay responses are policy overrides, not claims that LabDNS is authoritative for the entire zone. Overlay never returns NXDOMAIN or NODATA.

`resolver.Resolve` consumes a **pre-selected** zone ID. It does not rediscover the most-specific zone. Longest-suffix selection is `snapshot.ZoneIndex.Select` (used later by the DNS orchestrator). A name outside the selected zone's suffix is never NXDOMAIN; it is Fallthrough.

## Resolution order

1. Parse and validate the request.
2. Canonicalize QNAME and QTYPE.
3. Reject or constrain unsupported opcode, class, question count, and malformed EDNS.
4. Select the most specific local zone by suffix.
5. Select exact owner data.
6. Process CNAME according to the bounded chain rules.
7. If the exact owner does not exist, evaluate wildcard synthesis using the closest encloser.
8. If an authoritative zone matched and no answer exists, return NODATA or NXDOMAIN with SOA as appropriate.
9. If an overlay zone matched and no local answer exists, continue.
10. Select the most specific conditional forwarding policy, then the default `.` policy.
11. Check cache and exchange with upstreams as required.
12. Apply permitted response-phase chaos.
13. Encode a transport-correct response.

## Wildcard rules

A wildcard is a DNS owner name whose leftmost label is exactly `*`. It is not a shell glob, regular expression, or arbitrary pattern.

- Exact existing names take precedence over wildcard synthesis.
- Empty non-terminals count as existing names and can prevent a higher wildcard from matching.
- Synthesis uses the closest encloser and the corresponding wildcard source.
- The synthesized answer owner is the original QNAME, while the explanation reports the wildcard source owner.
- A literal query for an asterisk label is handled as a literal DNS name match.
- Wildcard DNAME is rejected.
- Wildcard NS is rejected in the initial release to avoid ambiguous delegation behavior.
- Wildcard CNAME is allowed with the same loop and coexistence rules as an exact CNAME.

Examples:

```text
Zone contents:
  *.tools.lab.example. A 10.42.0.20
  exact.tools.lab.example. A 10.42.0.21
  branch.tools.lab.example. TXT "exists"

Queries:
  alpha.tools.lab.example. A -> synthesized 10.42.0.20
  exact.tools.lab.example. A -> exact 10.42.0.21
  x.branch.tools.lab.example. A -> no match from *.tools because branch exists
```

## RRset and CNAME rules

- An owner with CNAME cannot have other ordinary data at that owner, apart from DNSSEC metadata if later supported.
- Configuration compilation rejects detectable CNAME loops. Runtime loops (for example via wildcard CNAME) return SERVFAIL.
- Runtime CNAME traversal has a strict configurable depth cap. `spec.defaults.cnameDepth` materializes to **8**. A zero `Snapshot.Defaults.CNAMEDepth` is **not** unlimited — `Resolve` falls back to 8.
- Overlay CNAME chains **may terminate in a forwarded name**. When the next target is outside the selected zone's local data, `Resolve` includes the CNAME chain, sets `Fallthrough=true`, and stops. It does not call the forwarder. Authoritative CNAME that leaves the zone is returned as a CNAME answer (NOERROR, no SOA) and does **not** fall through.
- When the CNAME target is still inside the selected authoritative zone, the final name’s RCODE applies: in-zone NXDOMAIN is NXDOMAIN + CNAME + SOA; in-zone NODATA is NOERROR + CNAME + SOA. Overlay still Fallthroughs those cases instead of synthesizing a negative.
- QTYPE CNAME returns the CNAME and does not follow. Other types follow in-zone CNAME targets, bounded by the depth cap.
- Multiple values of the same owner, type, class, and TTL form one RRset.
- An RRset uses one effective TTL after normalization.
- Record ordering is deterministic unless a configured answer-order or chaos policy changes it.

## Supported RR types

First-GA structured types: `A`, `AAAA`, `CNAME`, `TXT`, `MX`, `SRV`, `PTR`, `CAA`, `NS`, `SOA`, `SVCB`, `HTTPS`, plus validated generic RDATA (`typeCode` + presentation).

Known exclusions in this release:

- Wildcard `NS` and wildcard `DNAME` are rejected at validate and again at `resolver.Compile` (fail-closed).
- DNAME is not processed as a synthesis rule even when stored as generic RDATA.
- `Resolve` does not implement AXFR/IXFR (admission returns NOTIMP).

## Negative answers

- NXDOMAIN means the owner name does not exist.
- NODATA means the owner exists but the requested type does not.
- Authoritative negative answers include the zone SOA.
- Negative cache TTL follows SOA and configured bounds.
- Injected negative chaos responses are marked in explanation and telemetry and must still be syntactically correct.

## Forwarding

- Longest suffix wins among forwarding policies.
- A default policy uses suffix `.`.
- Upstream pools support ordered, round-robin, random, and health-aware strategies as separately documented.
- UDP truncation from an upstream triggers TCP retry when policy permits.
- NXDOMAIN does not normally trigger failover to another upstream.
- Timeouts, transport errors, SERVFAIL, and REFUSED failover behavior are explicit policy fields.
- Detect obvious self-forwarding and cyclic configurations during validation.

## Cache behavior

- Cache keys include all request attributes that materially affect the answer, including DNSSEC-related flags if supported.
- Positive and negative TTLs are clamped to configured bounds.
- Cache entries retain enough metadata to explain source, age, and upstream.
- Local-state revisions invalidate or namespace local answer caches so a mutation cannot return an old local override.
- Chaos can bypass, force-miss, or deliberately serve configured stale data only when enabled and bounded.

## DNS flags

- Echo the request ID and relevant question.
- Set QR on responses.
- Set AA only for authoritative local answers and authoritative negative answers. Overlay hits are not AA. Forwarded answers are not AA.
- Set RA only when forwarding/recursive service is available to the requesting client. `resolver.Resolve` leaves RA cleared; the orchestrator sets it later.
- Respect RD as a request signal but enforce local policy regardless. The transport echoes RD.
- Do not set AD on synthesized or other local data. First GA never forges AD.
- Clear CD on every local `Resolve` result. CD is passed through only on forwarded queries/responses (forwarder, not this package).
- Set TC only when response truncation is real or an explicit safe chaos action requests it.

Confirmed local matrix (RES-001): zone mode ∈ {authoritative, overlay} × RD ∈ {0,1} × CD ∈ {0,1}. AA follows the rules above; AD=0; CD=0; RA=0.

## Transport behavior

UDP and TCP return semantically equivalent answers unless:

- UDP size constraints require truncation.
- A transport-specific chaos action is explicitly selected.
- The client disconnects or cancels a TCP operation.

TCP handlers must use read, write, idle, and total request bounds. Delayed chaos must be cancellable when the TCP connection closes.

### Parse and admission (DNS-001)

Implemented in `internal/dnsserver` using `internal/dnswire`. Malformed input never panics.

| Condition | Response |
|---|---|
| Empty datagram or fewer than 12 header octets | Drop |
| Unpack failure with a usable header | FORMERR (ID echoed; question echoed when parsed) |
| UDP length greater than `MaxUDPSize` (default 4096) | Drop |
| TCP length prefix 0 or greater than `MaxTCPSize` | Close the connection |
| QR=1 (a response sent as a query) | Drop |
| Opcode other than QUERY | NOTIMP |
| QDCOUNT = 0 or QDCOUNT > `MaxQuestions` (default 1) | FORMERR |
| QCLASS other than IN | NOTIMP |
| QTYPE AXFR or IXFR | NOTIMP |
| EDNS version other than 0 | BADVERS (header RCODE 0 + OPT EXTENDED-RCODE 16, OPT VERSION 0) |
| No EDNS on UDP | Responses capped at 512 octets; TC set if truncated |
| EDNS UDP size < 512 | Treated as 512 (RFC 6891) |
| EDNS UDP size > `MaxEDNSUDPSize` (default 4096) | Clamped |

Handler failures, panics (including hook panics), and a nil `Response` with `HintSend` are fail-closed to SERVFAIL. Context cancel (shutdown, query timeout, TCP max-age, TCP peer close) produces no answer.

## Explainability

`resolve:explain` returns:

- Canonical query.
- Client group and transport classification, with privacy-safe fields.
- Selected zone and mode.
- Exact, wildcard, negative, cache, or upstream source.
- Wildcard source and closest encloser when applicable.
- Forwarding policy, pool, and selected upstream when applicable.
- Matched chaos policy, deterministic decision inputs, actions, and clamping.
- Final answer summary and state revision.

`resolver.Resolve` already fills the local subset on every result (`Explanation.Query`, `ZoneID`, `ZoneMode`, `Source`, `WildcardSource`, `ClosestEncloser`, `Revision`). Later packages add client-group, forwarding, and chaos fields.

Example (RFC 4592 synthesis):

```text
QNAME=alpha.tools.lab.example.net. QTYPE=A
zone=lab-zone mode=authoritative
source=wildcard
wildcardSource=tools-wildcard-a
closestEncloser=tools.lab.example.net.
answer owner=alpha.tools.lab.example.net. A 10.42.0.20
AA=1 AD=0 CD=0 RA=0
```

Example (overlay CNAME leaving local data):

```text
QNAME=alias.vendor.example. QTYPE=A
zone=vendor-overlay mode=overlay
source=exact
answers=alias.vendor.example. CNAME outside.example.
Fallthrough=true
AA=0 AD=0 CD=0 RA=0
```

Explain does not write to the cache, consume a live chaos budget, or change policy state unless an explicit simulation option requests a modeled budget result.

## Failure modes

- Unsupported query: return NOTIMP or FORMERR as appropriate.
- Missing forwarding policy: return REFUSED or SERVFAIL according to deployment policy; never silently use host resolver configuration.
- CNAME depth exceeded: SERVFAIL with an internal error code and optional EDE.
- Upstream deadline exceeded: SERVFAIL, optional EDE, and bounded retry history in explanation.

## Security considerations

Restrict forwarding by client network. Do not reflect large unbounded additional sections. Bound record counts, response sizes, CNAME depth, and upstream attempts.

## Observability

Count resolution source, zone mode, RCODE, cache status, upstream result, wildcard synthesis, negative type, and transport. Record names are not metric labels.

## Testing strategy

Use table-driven tests from RFC wildcard examples, empty non-terminal cases, exact-over-wildcard cases, CNAME chains, NXDOMAIN/NODATA distinctions, forwarding suffix precedence, UDP/TCP equivalence, and flag correctness.

## Compatibility implications

Changing resolution order, wildcard rules, negative behavior, or flags is a breaking semantic change and requires an ADR plus release-note migration guidance.

## Open questions

- Exact DNSSEC validation behavior beyond the first-GA rule: never forge AD; clear CD on local answers; pass CD through on forward.

Resolved 2026-08-15: overlay CNAME chains **may** terminate in a forwarded name, bounded by the global CNAME depth cap (default 8).
