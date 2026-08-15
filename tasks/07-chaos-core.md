# CHA-001: Chaos Core

Status: not-started
Recommended owner: Chaos engine agent
Dependencies: CFG-001, RES-001, STA-001
Exclusive ownership: `internal/chaos` policy compiler, selectors, budgets, simulation

## Goal

Implement policy indexing, matching, deterministic decisions, schedules, weighted outcomes, action validation, global safety budgets, simulation, and emergency disable without yet implementing every effect.

## Work items

- [ ] Compile policies by record, wildcard source, owner, zone, forwarding rule, upstream pool, client group, and global scope.
- [ ] Implement precedence, priority, compose, terminal, and exclusive-group behavior.
- [ ] Implement deterministic selector `hash-v1` with documented inputs.
- [ ] Implement random selector behind an injected decision source.
- [ ] Implement weighted outcome selection.
- [ ] Implement start, expiry, duration-after-activation, periodic flap, and every-Nth gates using fake-clock-friendly interfaces.
- [ ] Implement global caps and per-policy requested budgets.
- [ ] Implement delayed-request concurrency reservation interfaces.
- [ ] Implement protected name/client checks.
- [ ] Implement activation/deactivation and emergency-disabled state as snapshot data.
- [ ] Implement simulation that never sleeps or mutates.
- [ ] Implement explanation records for matched/skipped policies, decisions, outcomes, and clamping.
- [ ] Add audit event domain structures.

## Required tests

- [ ] Golden vectors for `hash-v1` across platforms.
- [ ] Same inputs reproduce across process restarts.
- [ ] Different seeds and policy IDs produce expected divergence.
- [ ] Weighted selection statistical bounds tests.
- [ ] Fake-clock start/expiry/flap tests.
- [ ] Precedence and composition matrix.
- [ ] Protected-object tests.
- [ ] Cap clamp/reject tests.
- [ ] Concurrent budget reservation and release tests.
- [ ] Emergency disable prevents new selections.
- [ ] Simulation has no sleeps, cache writes, budget consumption, or state changes.
- [ ] Fuzz policy combinations and selector inputs.
- [ ] Regression test for every discovered chaos-core bug.

## Documentation updates

- [ ] Record exact deterministic algorithm and golden vectors.
- [ ] Finalize selector and schedule schema.
- [ ] Update compatibility and error docs.
- [ ] Add release-note entry for chaos policy foundation.

## Acceptance criteria

- A policy attached to an exact or wildcard record can be matched and produce an explained outcome.
- Safety caps and protected objects cannot be bypassed by policy configuration.
- Deterministic simulation is stable and side-effect free.

## Handoff

Provide a stable action plan passed to CHA-002 and interfaces for resolver, forwarder, cache, and transport phases.
