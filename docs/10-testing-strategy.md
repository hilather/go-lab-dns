# Testing Strategy

Status: Proposed normative quality gate
Owners: All teams
Last reviewed: 2026-08-15

## Policy

Every area must have regression tests. No feature, bug fix, protocol behavior, configuration rule, operational script, or release process change is complete without automated coverage appropriate to its risk.

## Test layers

### Unit tests

- Canonical names and records.
- Zone and forwarding selection.
- Wildcard closest-encloser behavior.
- CNAME and negative answer logic.
- Cache TTL and key behavior.
- Chaos selectors, outcome selection, actions, and safety clamping.
- Authorization decisions.
- Error mappings.

### Property tests

- Canonicalization idempotence.
- Compile then export then compile semantic equivalence.
- Snapshot immutability.
- Weighted selection bounds.
- TTL values never exceed configured and wire bounds.
- No policy can exceed global delay/drop/concurrency caps.

### Fuzz tests

- DNS packet parsing.
- YAML and JSON decoding.
- RDATA parsing.
- Change operations.
- REST and MCP schema inputs.
- Chaos policy combinations.

Fuzzers must enforce memory and time limits and preserve minimized regressions in the corpus.

### Protocol integration tests

Run real UDP and TCP listeners and verify wire responses with an independent client library or tool. Cover:

- Exact and wildcard answers.
- Empty non-terminals.
- Authoritative NXDOMAIN and NODATA.
- Overlay fallthrough.
- CNAME chains and loops.
- Forwarding and TCP retry after truncation.
- EDNS handling.
- Chaos delay, drop, truncation, RCODE, TTL, alternate answer, and TCP close/reset.

### REST tests

- OpenAPI contract.
- Authentication and authorization.
- Planning, apply, reset, export, revision conflicts, and idempotency.
- Request size, deadline, and rate limits.
- Stable error codes.

### MCP tests

- Pinned protocol conformance.
- Streamable HTTP and optional stdio framing.
- Origin validation.
- Tool/resource schemas.
- Structured results and errors.
- Cancellation.
- Authorization.

### Parity tests

For each capability:

1. Invoke the shared domain handler directly.
2. Invoke REST with equivalent input.
3. Invoke MCP with equivalent input.
4. Normalize transport envelopes.
5. Assert equivalent domain output, error code, authorization result, revision behavior, and audit event.

### Race and leak tests

- Snapshot swaps under concurrent DNS load.
- Cache access.
- Upstream health changes.
- Delayed chaos cancellation.
- TCP disconnects.
- MCP cancellation.
- Idempotency cache.

Use Go race detection and explicit goroutine/resource leak assertions.

### Load and soak tests

Measure baseline and chaos-enabled overhead. Include:

- UDP and TCP QPS.
- Mixed local/cache/upstream traffic.
- Long-tail delays at maximum permitted concurrency.
- Upstream outage.
- Repeated state swaps.
- 24-hour or longer soak before GA where infrastructure permits.

### Container and deployment tests

- Non-root execution.
- Read-only filesystem.
- Port mapping.
- Read-only bootstrap mount.
- Reset behavior.
- Signal handling and graceful shutdown.
- Health checks.
- No hidden persistent state after recreation.

### Documentation tests

- Internal link checker.
- Markdown lint.
- Tested configuration examples.
- Tested command snippets where practical.
- Generated API and MCP documentation freshness.
- Required document metadata.

## Regression workflow

For every defect:

1. Capture a minimal failing test or fixture.
2. Confirm the test fails on the defective revision where practical.
3. Apply the fix.
4. Confirm the regression test and broader suite pass.
5. Document behavior changes and release-note impact.

## CI stages

Suggested required stages:

```text
format and generated-file check
static analysis and lint
unit and property tests
race tests
fuzz smoke tests
config compatibility
REST contract
MCP conformance
REST/MCP parity
DNS integration UDP/TCP
container and security tests
documentation checks
release-diff checks on tags
```

## CI failure hardening

A failing CI run is not dismissed as merely transient without evidence. Fix the cause and harden the relevant layer:

- Add a regression test or assertion.
- Make fixtures deterministic.
- Add explicit deadlines and cleanup.
- Improve failure artifacts and logs.
- Pin dependencies or test images.
- Remove shared mutable state.
- Use retries only for a proven external transient, with bounded attempts and diagnostics.

Never use broad retries to conceal a race or flaky assertion.

## Chaos-specific acceptance tests

- Per-record fixed delay falls within timing tolerance.
- Uniform delay stays within bounds and distribution checks pass.
- Deterministic mode repeats decisions across process restarts for the same policy algorithm and inputs.
- Emergency disable prevents new faults immediately.
- Protected names and clients are never faulted.
- Delay cancellation releases concurrency budget.
- Maximum delayed requests cannot be exceeded.
- UDP drop sends no response.
- Forced truncation causes client TCP retry.
- TCP reset does not crash or leak.
- Base and final answers are visible in explanation.

## Release test evidence

A release candidate records:

- Commit and dependency lock.
- Full CI result.
- Container digest.
- Schema and capability diff from previous tag.
- DNS behavior regression summary.
- Security scan results.
- Known limitations.

## Compatibility implications

Removing coverage for a public behavior is itself a compatibility risk and requires review. Golden fixtures are versioned and intentionally updated with explanatory release notes.
