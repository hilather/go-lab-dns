# PERF-001: Performance, Soak, and Interoperability

Status: done
Recommended owner: Performance/QA agent
Dependencies: feature-complete candidate
Exclusive ownership: benchmarks, load harness, interoperability matrix, performance baselines

## Goal

Validate that LabDNS remains correct, bounded, and operational under realistic load, upstream failures, state swaps, and chaos delays.

## Work items

- [x] Define baseline latency, throughput, memory, CPU, and connection targets for reference hardware.
- [x] Benchmark exact, wildcard, negative, cache hit, cache miss, upstream, and chaos paths.
- [x] Measure overhead with chaos configured but not triggered.
- [x] Load test maximum permitted delayed-request concurrency.
- [x] Soak test repeated state swaps and policy activation/expiry.
- [x] Test upstream outage and partial recovery.
- [x] Test query floods, randomized names, EDNS size pressure, and TCP connection pressure.
- [x] Run interoperability with common client resolvers and diagnostic tools on target operating systems.
- [x] Verify forced UDP truncation causes expected TCP behavior in representative clients.
- [x] Verify TTL, NXDOMAIN/NODATA, wildcard, CNAME, and EDE interpretation.
- [x] Record capacity-planning guidance and safe default limits.

## Required tests

- [x] Reproducible benchmark harness with pinned environment metadata.
- [x] No unbounded memory growth during soak.
- [x] No goroutine/file-descriptor leak during delay/drop/reset scenarios.
- [x] State swaps do not produce partial answers.
- [x] Emergency disable remains responsive under load.
- [x] Performance regression thresholds run in suitable CI or scheduled infrastructure.
- [x] Interoperability fixtures become regression tests for discovered differences.

## Documentation updates

- [x] Publish benchmark methodology and results.
- [x] Update deployment resource guidance and limits.
- [x] Update compatibility matrix and known limitations.
- [x] Add release-note entry for material performance or compatibility changes.

## Acceptance criteria

- Reference targets are met or explicitly adjusted with review.
- No known resource leak remains.
- Common lab clients correctly handle the supported response and chaos behaviors.

## Handoff

- Benches: `go test ./benches -bench=. -benchmem`
- Short CI: `make test-integration` (default soak 2s)
- Long soak: `go test ./internal/perf -soak=30m` or `LABDNS_SOAK_DURATION=30m`
- Fixtures: `testdata/interop/cases.json` + `testdata/interop/config.yaml`
- Capacity: `docs/11-deployment.md` § Capacity planning
- Client matrix: `docs/16-compatibility-and-versioning.md`
