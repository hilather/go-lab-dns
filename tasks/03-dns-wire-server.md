# DNS-001: DNS Wire Server

Status: not-started
Recommended owner: DNS transport agent
Dependencies: FND-001 and agreed query/response model interfaces from CFG-001
Exclusive ownership: `internal/dnswire`, `internal/dnsserver`

## Goal

Provide robust UDP and TCP DNS listeners, parse/encode adapters, admission limits, cancellation, and graceful shutdown without embedding resolution policy.

## Work items

- [ ] Select and pin the DNS library behind `internal/dnswire`.
- [ ] Define internal query and response structures independent of library types.
- [ ] Implement UDP and TCP listeners with shared request pipeline interfaces.
- [ ] Add request context, deadlines, source classification hook, and snapshot acquisition hook.
- [ ] Enforce packet, question count, EDNS, response, and connection limits.
- [ ] Implement correct DNS flags and error responses for parse/admission failures.
- [ ] Implement TCP framing, idle/read/write/total deadlines, and graceful close.
- [ ] Add shutdown behavior that stops accepts and cancels outstanding work.
- [ ] Expose transport actions needed by chaos: send, UDP drop, UDP truncate, TCP close, TCP reset, bounded no-response.
- [ ] Ensure transport actions cannot be called after response ownership is released.
- [ ] Add hooks for metrics without logging raw query names by default.

## Required tests

- [ ] Independent client sends and receives UDP and TCP queries.
- [ ] Malformed packets return or drop according to documented behavior without panic.
- [ ] Oversized and multi-question requests are bounded.
- [ ] EDNS size behavior is tested.
- [ ] TCP partial reads, disconnects, deadline expiry, and connection caps are tested.
- [ ] Graceful shutdown drains or cancels within deadline.
- [ ] UDP drop sends no packet.
- [ ] UDP forced truncation produces a valid TC response.
- [ ] TCP close/reset does not crash or leak.
- [ ] Race and goroutine-leak tests pass.
- [ ] Fuzz corpus covers parser and adapter boundaries.
- [ ] Regression test is added for every discovered transport bug.

## Documentation updates

- [ ] Document selected library and adapter boundary.
- [ ] Record listener defaults and limits.
- [ ] Update security and deployment docs if platform behavior differs.
- [ ] Add release-note entry for DNS transport support.

## Acceptance criteria

- Listeners are usable with a stub resolver handler.
- UDP/TCP lifecycle, bounds, and cancellation pass tests.
- Third-party DNS types do not escape the adapter.
- No resolver or chaos policy is embedded in transport code.

## Handoff

Document the handler interface, response ownership rules, context lifetime, and chaos transport hooks.
