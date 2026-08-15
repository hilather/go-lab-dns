# DNS Semantics

Status: Implementation-confirmed for local resolution (RES-001) and forwarding/cache/orchestrator (FWD-001)
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

`resolver.Resolve` consumes a **pre-selected** zone ID. It does not rediscover the most-specific zone. Longest-suffix selection is `snapshot.ZoneIndex.Select` (used by `dnsquery`). A name outside the selected zone's suffix is never NXDOMAIN; it is Fallthrough.

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
- Overlay CNAME chains **may terminate in a forwarded name**. When the next target is outside the selected zone's local data, `Resolve` includes the CNAME chain, sets `Fallthrough=true`, and stops. It does not call the forwarder. `dnsquery` then re-selects the forwarding policy on the CNAME target for `Exchange` (classification / future chaos still use the original QNAME's policy). Authoritative CNAME that leaves the zone is returned as a CNAME answer (NOERROR, no SOA) and does **not** fall through.
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

Implemented in `internal/forwarder` + `internal/dnsquery`. `forwarder.Exchange` consumes a **pre-selected** policy ID. Longest-suffix selection is `snapshot.ForwardingIndex.Select`.

- Longest suffix wins among forwarding policies. The root suffix `.` matches every name and ranks lowest.
- There is **no host-resolver fallback**. Only configured `host:port` endpoints are dialed.
- UDP truncation from an upstream triggers a TCP retry to the **same** endpoint when `failover.udpTruncateRetryTCP` is true. The Go zero value is false (no retry).
- NXDOMAIN (and other successful RCODEs except SERVFAIL/REFUSED) do **not** fail over.
- Timeouts, transport errors, SERVFAIL, and REFUSED failover are explicit `FailoverSpec` bools. They are **not** materialized: the Go zero value means do not fail over. A zero `timeout` is **not** unlimited; Exchange uses a **500ms** per-attempt budget so it stacks under the 2s query-handler total deadline (at least one failover try remains when `onTimeout` is set). Dial uses a 250ms connect budget capped by the remaining attempt time. If the parent deadline expires mid-attempt, Exchange returns the context error (no synthesized SERVFAIL).
- Overlay CNAME that leaves local data re-selects the forwarding policy on the **CNAME target** for the upstream exchange. Classification (and a future chaos `Decide`) still sees the policy selected from the original QNAME.
- Self-forwarding and cyclic configurations are rejected at `config.Validate` (they need the listen address).
- Forwarded answers never set AA or AD. CD is passed through. RA is left false for the orchestrator.

### Pool strategies

| Strategy | First pick | Failover walk |
|---|---|---|
| `ordered` | configured index 0 | remaining configured order |
| `round-robin` | process-local counter per pool | remaining from that start |
| `random` | injected RNG (tests use a seed) | remaining from that start |
| `health-aware` | first currently healthy (or cooldown-expired) member in configured order | remaining healthy first, then last-resort unhealthy |

Health is **query-driven**: no extra probe packets. An upstream is marked down after **2** consecutive timeout or transport failures and becomes probe-eligible after a **30s** cooldown. SERVFAIL/REFUSED RCODEs do not change health.

### Refuse-forward (unknown / local-only clients)

`spec.access.unknownClient` is `refuse-forward` (refuse *recursion*, not all DNS). A source IP that matches no `clientGroups[].cidrs`:

1. is classified `ClientGroupID = ""`;
2. still receives local authoritative or overlay answers (including authoritative NXDOMAIN/NODATA) with **RA=0**;
3. is **never forwarded** and does not fill cache from upstream;
4. receives **REFUSED** (RA=0) only when there is no local path (no matching zone, or overlay fallthrough with no permitted policy).

`ClientGroup.AllowForward=false` is the same gate for a known group. Empty `clientGroups` serves local zones to everyone and forwards to no one. `AccessIndex` fill is STA-001; until then `dnsquery` classifies from `snap.Canonical.Spec.Access` (longest-prefix CIDR).

RD=1 does **not** grant forwarding to unknown or local-only clients. RD=0 does **not** suppress configured forwarding for a known `AllowForward` group.

## Cache behavior

Implemented in `internal/cache` (process-scoped; **not** a Snapshot field).

- Keys include Revision, QNAME, QTYPE, QCLASS, a local/upstream bit, and (for upstream) CD + forwarding policy ID. Revision namespaces so a mutation cannot return a pre-swap local override. Unknown/local-only clients never look up or fill upstream entries.
- Positive TTLs are clamped to `[minimumTTL, maximumTTL]` when those bounds are > 0. Negative TTLs are capped by `maximumNegativeTTL`. A zero bound means no clamp on that side. A clamped TTL of 0 is not stored.
- Get returns a **copy** whose RR TTLs are `min(storedTTL − elapsed, ExpireAt − now)` (floor 0). `ExpireAt` is the clamped lifetime, so a `maximumTTL` cap cannot be advertised past. Chaos hooks (`bypass`, `force-miss`, `serve-stale`, skip-put) change the request path or the returned copy; CHA-002 wires them.
- SERVFAIL/REFUSED/FORMERR/NOTIMP are not cached. Overlay `Fallthrough` results are not cached until a forward completes.

## DNS flags

- Echo the request ID and relevant question.
- Set QR on responses.
- Set AA only for authoritative local answers and authoritative negative answers. Overlay hits are not AA. Forwarded answers are not AA.
- Set RA only when forwarding is available to this client: a matching group, `AllowForward`, and a selected forwarding policy. `resolver.Resolve` and `forwarder.Exchange` leave RA cleared; `dnsquery` sets it.
- Respect RD as a request signal but enforce local policy regardless. The transport echoes RD.
- Do not set AD on synthesized or other local data. First GA never forges AD.
- Clear CD on every local `Resolve` result. CD is passed through only on forwarded queries/responses (forwarder, not this package).
- Set TC only when response truncation is real or an explicit safe chaos action requests it.

Confirmed local matrix (RES-001): zone mode ∈ {authoritative, overlay} × RD ∈ {0,1} × CD ∈ {0,1}. AA follows the rules above; AD=0; CD=0; RA=0.

Confirmed orchestrator matrix (FWD-001): zone mode ∈ {authoritative, overlay, none} × RD ∈ {0,1} × client ∈ {known-forward, known-local-only, unknown} × CD ∈ {0,1}. RA is 1 only for known-forward with a selected policy (including local answers to those clients). AD=0. CD is 0 on local answers and passed through on forwarded answers.

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
