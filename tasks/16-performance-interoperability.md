# PERF-001: Performance, Soak, and Interoperability

Status: not-started
Recommended owner: Performance/QA agent
Dependencies: feature-complete candidate
Exclusive ownership: benchmarks, load harness, interoperability matrix, performance baselines

## Goal

Validate that LabDNS remains correct, bounded, and operational under realistic load, upstream failures, state swaps, and chaos delays.

## Work items

- [ ] Define baseline latency, throughput, memory, CPU, and connection targets for reference hardware.
- [ ] Benchmark exact, wildcard, negative, cache hit, cache miss, upstream, and chaos paths.
- [ ] Measure overhead with chaos configured but not triggered.
- [ ] Load test maximum permitted delayed-request concurrency.
- [ ] Soak test repeated state swaps and policy activation/expiry.
- [ ] Test upstream outage and partial recovery.
- [ ] Test query floods, randomized names, EDNS size pressure, and TCP connection pressure.
- [ ] Run interoperability with common client resolvers and diagnostic tools on target operating systems.
- [ ] Verify forced UDP truncation causes expected TCP behavior in representative clients.
- [ ] Verify TTL, NXDOMAIN/NODATA, wildcard, CNAME, and EDE interpretation.
- [ ] Record capacity-planning guidance and safe default limits.

## Required tests

- [ ] Reproducible benchmark harness with pinned environment metadata.
- [ ] No unbounded memory growth during soak.
- [ ] No goroutine/file-descriptor leak during delay/drop/reset scenarios.
- [ ] State swaps do not produce partial answers.
- [ ] Emergency disable remains responsive under load.
- [ ] Performance regression thresholds run in suitable CI or scheduled infrastructure.
- [ ] Interoperability fixtures become regression tests for discovered differences.

## Documentation updates

- [ ] Publish benchmark methodology and results.
- [ ] Update deployment resource guidance and limits.
- [ ] Update compatibility matrix and known limitations.
- [ ] Add release-note entry for material performance or compatibility changes.

## Acceptance criteria

- Reference targets are met or explicitly adjusted with review.
- No known resource leak remains.
- Common lab clients correctly handle the supported response and chaos behaviors.

## Handoff

Provide benchmark baselines, test environment, capacity recommendations, and unresolved client differences.
