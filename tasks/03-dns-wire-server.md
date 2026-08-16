# DNS-001: DNS Wire Server

Status: done
Recommended owner: DNS transport agent
Dependencies: FND-001 and agreed query/response model interfaces from CFG-001
Exclusive ownership: `internal/dnswire`, `internal/dnsserver`

## Goal

Provide robust UDP and TCP DNS listeners, parse/encode adapters, admission limits, cancellation, and graceful shutdown without embedding resolution policy.

## Work items

- [x] Select and pin the DNS library behind `internal/dnswire`.
- [x] Define internal query and response structures independent of library types.
- [x] Implement UDP and TCP listeners with shared request pipeline interfaces.
- [x] Add request context, deadlines, source classification hook, and snapshot acquisition hook.
- [x] Enforce packet, question count, EDNS, response, and connection limits.
- [x] Implement correct DNS flags and error responses for parse/admission failures.
- [x] Implement TCP framing, idle/read/write/total deadlines, and graceful close.
- [x] Add shutdown behavior that stops accepts and cancels outstanding work.
- [x] Expose transport actions needed by chaos: send, UDP drop, UDP truncate, TCP close, TCP reset, bounded no-response.
- [x] Ensure transport actions cannot be called after response ownership is released.
- [x] Add hooks for metrics without logging raw query names by default.

## Required tests

- [x] Independent client sends and receives UDP and TCP queries.
- [x] Malformed packets return or drop according to documented behavior without panic.
- [x] Oversized and multi-question requests are bounded.
- [x] EDNS size behavior is tested.
- [x] TCP partial reads, disconnects, deadline expiry, and connection caps are tested.
- [x] Graceful shutdown drains or cancels within deadline.
- [x] UDP drop sends no packet.
- [x] UDP forced truncation produces a valid TC response.
- [x] TCP close/reset does not crash or leak.
- [x] Race and goroutine-leak tests pass.
- [x] Fuzz corpus covers parser and adapter boundaries.
- [x] Regression test is added for every discovered transport bug.

## Documentation updates

- [x] Document selected library and adapter boundary.
- [x] Record listener defaults and limits.
- [x] Update security and deployment docs if platform behavior differs.
- [x] Add release-note entry for DNS transport support.

## Acceptance criteria

- Listeners are usable with a stub resolver handler.
- UDP/TCP lifecycle, bounds, and cancellation pass tests.
- Third-party DNS types do not escape the adapter.
- No resolver or chaos policy is embedded in transport code.

## Handoff

**Handler** (`internal/dnsserver`):

```go
ServeDNS(ctx context.Context, req *model.Query) (*Response, TransportHint, error)
```

`ctx` is canceled on graceful shutdown, `QueryTimeout`, TCP max-age, or a detected TCP peer close. The listener attaches peer address and transport (`PeerAddr`, `TransportFromContext`). Optional `AcquireSnapshot` and `ClassifySource` hooks may annotate `ctx` after admission; the transport does not inspect them.

**Response ownership:** `SetHint` / `SetHoldFor` / `SetResult` are valid only until `ServeDNS` returns. The server then `Release`s the `Response`. Further transport actions return `ErrReleased` and are ignored. The returned `TransportHint` wins unless it is `HintSend`, in which case a non-send hint stored on `Response` is used.

**Transport hints:** `Send`, `Drop`, `Truncate`, `TCPClose`, `TCPReset`, `HoldThenClose`. TCP-only hints on UDP are **drop**. `Truncate` on TCP is **send** of the full response. Unknown hints are **drop**. Hold is clamped to `MaxHold` (default 1s).

**Library pin:** `github.com/miekg/dns v1.1.72` in `internal/dnswire` only. Import tests fail if any other package imports it.

**Limits and admission:** see `docs/01-architecture.md` § DNS wire adapter and listeners and `docs/02-dns-semantics.md` § Parse and admission.
