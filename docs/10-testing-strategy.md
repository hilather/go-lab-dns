# Testing Strategy

Status: Proposed normative quality gate
Owners: All teams
Last reviewed: 2026-09-01 (resolve useCache fallthrough must not poison DNS cache)
Last reviewed: 2026-08-29 (app tsc excludes Vitest files; web-test runs tsc --noEmit)
Last reviewed: 2026-08-29 (operator chrome leftover-color grep and per-route class assertions)
Last reviewed: 2026-08-29 (grouped nav, single emergency verb, zones inventory/hops)
Last reviewed: 2026-08-19 (Playwright operator matrix; testdata/web fixture)
Last reviewed: 2026-08-19 (openapi-fetch client; revision-aware query keys)
Last reviewed: 2026-08-19 (make web-test / web-build and CI job web)
Last reviewed: 2026-08-15 (REL-001 release-diff and required CI)
Last reviewed: 2026-08-15 (PERF-001 benches, soak, interop)
Last reviewed: 2026-08-15 (GIT-001 deployment template probes)

## Policy

Every area must have regression tests. No feature, bug fix, protocol behavior, configuration rule, operational script, or release process change is complete without automated coverage appropriate to its risk.

## Test layers

### Unit tests

- Canonical names and records.
- Zone and forwarding selection.
- Wildcard closest-encloser behavior.
- CNAME and negative answer logic.
- Cache TTL and key behavior. Management resolve `useCache` must not store overlay Fallthrough; DNS lookup ignores a poisoned fallthrough local entry.
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

DNS-001 lands independent localhost UDP/TCP client tests in `internal/dnsserver` (no miekg import) plus a `dnswire` fuzz corpus under `testdata/packets` and `internal/dnswire/testdata/fuzz/FuzzParse`. Cover:

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

### Operator UI tests

`make web-test` runs `tsc --noEmit` then Vitest in `web/` (fail closed if Node is missing; not Playwright) and fails if committed `web/src/api/openapi.d.ts` is stale versus `api/openapi/v1.json`. App `web/tsconfig.json` excludes `src/**/*.test.ts` and `src/**/*.test.tsx` so Docker `npm run build` does not typecheck Vitest files (those stay on Vitest). `make web-generate` writes that file; `make generate` stays Go-only. `make web-build` emits `web/dist` only. GitHub job id `web` is required. Session-memory and login tests must prove CSRF and bearer tokens are not written to `localStorage`, `sessionStorage`, IndexedDB, or the URL. Query-key tests must prove snapshot keys include revision and invalidate on revision change without touching live upstream/cache/chaos keys. Shell/nav tests cover Inspect/Mutate/Ref grouping (including Reset). Emergency-control tests cover a single visible verb plus ScopeGate. Zones tests cover the FQDN-first inventory, records columns, and `/changes` hops with raw GET JSON. Chrome tests read operator CSS for leftover `#ccc` / Segoe / Google Fonts and assert charcoal/amber tokens on both `.login` and `.shell`; page tests assert `page-lede` / `surface` / `data-table` / `code-block` / `btn-accent` on the leftover routes. Login contrast remains a Playwright check (`web/e2e/a11y.spec.ts`). REST/cmd tests prove non-loopback `GET /` is 200 HTML without a bearer when the UI is enabled, and `GET /v1/state` is 401.

Operator Playwright (`make web-e2e` / `npm run test:e2e`) talks to loopback `labdns serve` with [`testdata/web/`](https://github.com/hilather/go-lab-dns/blob/main/testdata/web) (pack-sample plus viewer vs admin tokens). Job `web` installs Chromium via `npx playwright install --with-deps chromium` and runs the matrix after Vitest/`web-build`. Assertions are DOM and HTTP, not screenshot diffs. The operator scenario restores bootstrap at the start of each attempt so a CI retry does not plan/apply against a dirty snapshot.

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

#### PERF-001 harness

| Surface | Location | CI |
|---|---|---|
| Path benches (exact, wildcard, negative, cache hit/miss, upstream, chaos triggered, chaos idle) | `benches` (`go test ./benches -bench=. -benchmem`) | compile + `TestHarnessPaths` only; numbers are not gates |
| Max delayed concurrency, delay cancel leak | `internal/perf` | yes |
| Soak of snapshot swaps + chaos activate/expiry | `internal/perf` `-soak` (default **2s**) | yes, short |
| Flood / admission (random names, EDNS oversize, inflight, TCP cap) | `internal/perf` | yes |
| Upstream outage + recovery | `internal/perf` | yes |
| Emergency disable under load | `internal/perf` | yes |
| Client fixtures | [`testdata/interop`](https://github.com/hilather/go-lab-dns/blob/main/testdata/interop) + `internal/interop` | yes (`dig` skipped if absent) |

Pinned environment metadata (`goos`, `goarch`, `go version`, `NumCPU`, `GOMAXPROCS`, reference hardware class) is printed by `TestEnv` / `TestEnvMetadataPinned`. Reference class is **2 vCPU / 4 GiB** CI-runner or developer laptop.

Long soak (pre-GA): `go test ./internal/perf -soak=30m` or `LABDNS_SOAK_DURATION=30m`. `make test-integration` runs the short suite (`./internal/interop ./internal/perf ./benches`, 90s timeout).

Soak assertions: every wire answer is a complete old or new RRset (never mixed/partial), delay reservations return to zero, cache stays ≤ `maxEntries`, goroutine count does not grow without bound.

### Container and deployment tests

- Non-root execution.
- Read-only filesystem.
- Port mapping.
- Read-only bootstrap mount.
- Reset behavior.
- Signal handling and graceful shutdown.
- Health checks.
- No hidden persistent state after recreation.
- GitOps template (`examples/labdns-deploy`): digest pin, policy rejection
  (broad CIDRs, unapproved upstreams, unsafe chaos), probe suite (exact,
  wildcard, authoritative miss, overlay, unknown-client RA=0 / REFUSED,
  chaos simulation), rollback, and script fail-closed checks.

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
