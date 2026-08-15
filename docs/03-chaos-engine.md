# Bounded DNS Chaos Engine

Status: Proposed normative behavior
Owners: Chaos, DNS, Security
Last reviewed: 2026-08-15
Related ADRs: 0005, 0007

## Problem statement

Laboratory users need to test software behavior under slow, unreliable, intermittent, stale, truncated, or incorrect DNS conditions. Faults must be attachable to an individual DNS entry, including wildcard entries, while remaining reproducible, observable, reversible, and safe for the DNS service itself.

## Goals

- Set fixed delay, uniform delay, or other bounded distributed response delays per exact or wildcard RRset.
- Model useful DNS and transport failures without requiring external network emulators.
- Scope faults by record, owner, zone, client group, query type, transport, forwarding rule, upstream pool, and time window.
- Provide deterministic seeded behavior for repeatable tests.
- Protect the management plane and service resources.
- Make every decision explainable through REST and MCP.
- Allow temporary runtime experiments and Git-controlled permanent experiments.

## Non-goals

- Arbitrary malformed packet fuzzing in the production process.
- Kernel-level latency, bandwidth, reordering, or packet corruption for non-DNS traffic.
- Hiding chaos from operators.
- Unbounded sleeps, goroutine accumulation, or indefinite TCP connections.
- Using chaos as a security testing bypass.

## Invariants

1. Chaos is disabled by default.
2. Chaos never affects REST, MCP, metrics, liveness, readiness, or emergency disable.
3. Global limits override all individual policies.
4. Every live delayed operation is cancellable and counted against a budget.
5. Every policy has a stable ID and audit metadata.
6. A deterministic policy produces the same decision for the same documented decision key.
7. The base DNS result is available to explanation even when the final action drops or replaces it.
8. Protected names, protected clients, and management addresses cannot be affected.
9. Unsafe wire faults are excluded from the main service.
10. Reset or emergency disable immediately prevents new chaos actions and cancels cancellable outstanding delays.

## Core model

A chaos policy contains:

- Identity: ID, description, owner, labels, reason, ticket/reference.
- Activation: enabled, start time, expiry, optional periodic schedule.
- Scope: record IDs, owner names, wildcard source IDs, zones, forwarding policy IDs, upstream pool IDs, client groups, QTYPEs, and transports.
- Selector: trigger probability, deterministic mode, seed, sampling key, optional every-Nth or periodic gate.
- Outcomes: weighted alternatives, each containing ordered actions.
- Safety class: low, medium, high, or unsafe-deferred.
- Budget requests: maximum delay, concurrency, rate, and effect frequency.

A record links to one or more policies by stable ID:

```yaml
records:
  - id: tools-wildcard-a
    owner: "*.tools"
    type: A
    ttl: 30s
    values:
      - 10.42.0.20
    chaosPolicyRefs:
      - slow-tools
      - occasional-servfail
```

A policy linked to an RRset applies only when that RRset contributes to the base answer. Owner- or zone-scoped policies can apply to NODATA and NXDOMAIN cases.

## Scope precedence

Specific policies are evaluated before broad policies:

1. Exact record ID.
2. Synthesized wildcard source record ID.
3. Exact owner scope.
4. Zone scope.
5. Forwarding rule scope.
6. Upstream pool scope.
7. Client-group scope.
8. Global scope.

Precedence controls evaluation order, not automatic cancellation. A policy can declare one of:

- `compose`: continue evaluating compatible lower-precedence policies.
- `terminal`: stop after this policy selects an outcome.
- `exclusive-group`: only the highest-priority selected policy in the named group runs.

Conflicting terminal transport actions are rejected during candidate-state validation.

## Decision modes

### Deterministic

Use a stable pseudorandom value derived from:

```text
policy seed
state revision or explicit policy revision
policy ID
canonical QNAME
QTYPE
client-group ID or privacy-safe client bucket
transport
configured time bucket
optional caller-supplied simulation nonce
```

The implementation must document the hash and mapping algorithm and lock it with golden tests. Changing that algorithm is a compatibility change for deterministic experiments. CFG validation rejects `selector.timeBucket` values below `1s` so the `hash-v1` second-precision encoding cannot collapse windows.

### Random

Use a concurrency-safe cryptographic or statistically sound PRNG seeded at snapshot compilation. Random mode is appropriate for general resilience tests but is less reproducible.

### Sequence test mode

An in-process test harness may inject a deterministic decision source. Production APIs must not expose global mutable counters that make decisions depend on goroutine scheduling.

## Scheduling

Supported activation gates:

- Always while enabled.
- RFC3339 start and expiry.
- Duration after activation.
- Periodic flap window: period, unhealthy duration, and phase offset.
- Every Nth deterministic bucket for repeatable intermittent behavior.

Cron expressions are deferred to avoid timezone and missed-run ambiguity. Every runtime-created high-impact policy must have an expiry.

## Effect catalog

### 1. Delay and jitter

Use cases: slow resolver simulation, tail latency, regional latency, delayed upstream behavior.

Fields:

- Phase: before resolution, before upstream, after upstream, before response.
- Distribution: fixed and uniform in the first release; clipped normal, log-normal, and exponential may be added behind compatibility-tested schema fields.
- Fixed duration or minimum/maximum duration.
- Probability or weighted outcome selection.
- Maximum effective duration after global clamping.

Per-entry delay is normally applied in `before response` after the RRset is selected. Delay must use context-aware timers and release concurrency budget on cancellation.

### 2. RCODE and NODATA injection

Supported initial outcomes:

- SERVFAIL.
- REFUSED.
- NXDOMAIN.
- NODATA: NOERROR with empty answer and correct authority data when available.
- FORMERR and NOTIMP only for protocol-client testing and only with an explicit medium/high safety class.

An optional Extended DNS Error can explain an injected failure. EDE never changes the base RCODE meaning.

### 3. Silent drop or bounded no-response

- UDP: intentionally send no response.
- TCP: hold only until the configured bounded chaos timeout, then close gracefully or reset according to the selected action.
- Never retain an operation beyond the global request lifetime.
- Drop probability is capped globally.

### 4. TCP close or reset

Useful for testing resolver retry behavior. Applies only to TCP and must be implemented without panicking the server or leaking connections.

### 5. Forced truncation

For UDP, set TC and return a minimal syntactically valid response to force a TCP retry. The policy may optionally retain a limited answer prefix, but the default is a minimal truncated response. Forced truncation on TCP is invalid.

### 6. TTL manipulation

- Set TTL to a fixed value.
- Clamp TTL to a range.
- Add bounded deterministic or random TTL jitter.
- Force zero TTL.
- Manipulate negative TTL within safety bounds.

The final TTL cannot exceed global maximums or overflow wire fields.

### 7. Alternate answer

Replace or augment an RRset with explicitly configured values. This is useful for failover, migration, and misrouting tests.

Safety rules:

- Alternate values are fully validated at config compile time.
- Address values may be restricted to configured lab CIDRs.
- CNAME targets may be restricted to managed or explicitly allowed suffixes.
- An alternate answer cannot create an unbounded CNAME chain.
- The explanation always reports base and final answers.

### 8. Answer omission and partial answer

- Remove selected values from a multi-value RRset.
- Limit the answer to N values.
- Return an empty answer as NODATA only when explicitly selected.

### 9. Ordering and rotation

- Shuffle values deterministically or randomly.
- Rotate a selected first answer.
- Choose a weighted subset.

Ordering changes must not mutate the immutable stored RRset.

### 10. Flapping

A periodic selector alternates healthy and fault outcomes. Useful for intermittent service discovery failures. Flapping is derived from wall-clock buckets and an explicit phase offset, not mutable per-request state.

### 11. Cache behavior

Initial safe cache effects:

- Bypass cache lookup.
- Force cache miss without deleting the entry.
- Expire the selected entry early for the current request.
- Serve a stale entry only if a real stale copy exists and stale serving is enabled.

Cache chaos must not corrupt shared cache metadata. It changes the request path or returns a copy.

### 12. Upstream behavior

- Delay before an upstream exchange.
- Pretend a selected upstream is unavailable.
- Force selection of a named upstream.
- Force timeout or transport error.
- Force failover to the next configured upstream.
- Return a configured synthetic upstream RCODE.

These effects apply to LabDNS behavior, not to the actual upstream server.

### 13. Rate and concurrency pressure

- Delay or reject queries after a policy-scoped QPS threshold.
- Limit concurrent matching queries.
- Select drop, REFUSED, or SERVFAIL when the threshold is exceeded.

This is not a replacement for the global abuse rate limiter.

## Action phases

Actions execute in this fixed phase order:

1. Admission annotations and selector decision.
2. Pre-resolution delay or forced early failure.
3. Base local/cache/upstream resolution.
4. Response replacement, omission, ordering, TTL, and EDE changes.
5. Post-resolution delay.
6. Transport action: send, truncate, drop, close, or reset.

An outcome cannot schedule an action in an earlier phase after a later phase has run. Invalid combinations are rejected at configuration compile time.

## Global safety policy

Example:

```yaml
chaos:
  enabled: true
  emergencyDisabled: false

  safety:
    protectedNames:
      - dns.lab.example.
      - control.lab.example.
    protectedClientGroups:
      - management
      - monitoring
    allowedAddressCIDRs:
      - 10.0.0.0/8
      - 192.168.0.0/16
    maxDelay: 10s
    maxConcurrentDelayed: 2000
    maxDropProbability: 0.50
    maxActiveHighImpactPolicies: 5
    requireExpiryForSafetyClasses:
      - medium
      - high
    defaultMaxLifetime: 1h
```

The runtime may impose stricter command-line or environment caps that YAML cannot relax.

## Complete policy example

```yaml
chaos:
  policies:
    - id: slow-tools
      description: Add repeatable tail latency to wildcard tool records
      owner: platform-lab
      reason: Validate client startup deadlines
      enabled: true
      expiresAt: 2026-08-16T18:00:00Z
      safetyClass: low

      scope:
        recordIds:
          - tools-wildcard-a
        qtypes:
          - A
          - AAAA
        clientGroups:
          - test-devices
        transports:
          - udp
          - tcp

      selector:
        mode: deterministic
        seed: lab-startup-test-v1
        probability: 1.0
        timeBucket: 1s

      outcomes:
        - id: normal-tail
          weight: 90
          actions:
            - type: delay
              phase: before-response
              distribution: uniform
              min: 50ms
              max: 250ms

        - id: very-slow
          weight: 8
          actions:
            - type: delay
              phase: before-response
              distribution: fixed
              duration: 2s

        - id: fail
          weight: 2
          actions:
            - type: rcode
              value: SERVFAIL
              ede:
                code: 0
                text: lab-injected-failure
```

## Runtime API operations

- List and get policies.
- Validate or plan policy changes.
- Apply a policy change atomically with the rest of state.
- Activate or deactivate a policy.
- Extend or shorten expiry with authorization.
- Simulate a policy against one or more query contexts.
- Explain a live query decision.
- Emergency-disable all chaos.
- Export canonical policy state and deployment-repo patch operations.

REST and MCP expose the same semantics.

## Simulation

Simulation takes a state revision, query context, optional policy set, and optional nonce. It returns:

- Matching and skipped policies with reasons.
- Decision keys and reproducibility metadata.
- Selected outcome and actions.
- Safety clamping or rejection.
- Base answer if resolution simulation is requested.
- Final modeled answer or transport behavior.

Simulation does not sleep, send packets, change cache state, consume budgets, or activate policies.

## Emergency disable

Provide three independent controls:

1. Startup flag or environment override that disables chaos regardless of YAML.
2. Authenticated admin REST/MCP capability that disables new chaos actions.
3. Local signal or Unix-socket administrative path if the deployment requires recovery when HTTP auth is unavailable.

Disabling chaos swaps to a snapshot with chaos execution off and cancels outstanding context-aware delays where possible.

## Failure modes

- Invalid policy: reject candidate state.
- Requested delay exceeds cap: clamp or reject according to safety policy; report the result.
- Budget exhausted: skip the action or apply configured fallback; never block indefinitely.
- Timer canceled: release budget and stop processing.
- Policy expired: do not select it; emit a one-time or bounded event.
- Conflicting actions: reject at compile time.
- Clock jump: deterministic bucket behavior may change; use monotonic time for durations and wall time only for absolute schedules.

## Security considerations

Chaos activation is a privileged operation. Separate scopes should include `dns.chaos.read`, `dns.chaos.write`, `dns.chaos.activate`, and `dns.chaos.emergency`. Audit every state change and rejected activation. Restrict alternate addresses and CNAME targets. Never allow arbitrary shell, file, network, or packet-construction instructions through a chaos policy.

## Observability

Metrics should include policy match, trigger, selected outcome, skipped reason, clamped action, delay bucket, active delayed requests, budget use, expired policies, drops, resets, truncations, and synthetic RCODEs. Policy ID is allowed as a label only because configured policy count is bounded. Do not label by QNAME.

## Testing strategy

- Golden deterministic-decision tests.
- Probability distribution statistical tests with tolerant bounds.
- Fake-clock schedule tests.
- Cancellation and goroutine-leak tests.
- Race tests for snapshot swaps during delayed queries.
- UDP drop and truncation integration tests.
- TCP close/reset integration tests.
- TTL and alternate-answer packet tests.
- Safety cap and protected-name tests.
- REST/MCP parity tests for every chaos capability.
- Regression test for every chaos bug.

## Compatibility implications

Policy schemas, decision algorithms, action ordering, deterministic keys, and safety default changes are compatibility surfaces. Deterministic algorithm changes require a new algorithm identifier and migration notes.

## Open questions

- Whether clipped normal and log-normal distributions ship in the first release.
- Whether stale-answer experiments require a configurable snapshot/cache history.
- Whether a separate privileged `labdns-wirefuzz` companion should be designed later.
