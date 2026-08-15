# CHA-001: Chaos Core

Status: done
Recommended owner: Chaos engine agent
Dependencies: CFG-001, RES-001, STA-001
Exclusive ownership: `internal/chaos` policy compiler, selectors, budgets, simulation

## Goal

Implement policy indexing, matching, deterministic decisions, schedules, weighted outcomes, action validation, global safety budgets, simulation, and emergency disable without yet implementing every effect.

## Work items

- [x] Compile policies by record, wildcard source, owner, zone, forwarding rule, upstream pool, client group, and global scope.
- [x] Implement precedence, priority, compose, terminal, and exclusive-group behavior.
- [x] Implement deterministic selector `hash-v1` with documented inputs.
- [x] Implement random selector behind an injected decision source.
- [x] Implement weighted outcome selection.
- [x] Implement start, expiry, duration-after-activation, periodic flap, and every-Nth gates using fake-clock-friendly interfaces.
- [x] Implement global caps and per-policy requested budgets.
- [x] Implement delayed-request concurrency reservation interfaces.
- [x] Implement protected name/client checks.
- [x] Implement activation/deactivation and emergency-disabled state as snapshot data.
- [x] Implement simulation that never sleeps or mutates.
- [x] Implement explanation records for matched/skipped policies, decisions, outcomes, and clamping.
- [x] Add audit event domain structures.

## Required tests

- [x] Golden vectors for `hash-v1` across platforms.
- [x] Same inputs reproduce across process restarts.
- [x] Different seeds and policy IDs produce expected divergence.
- [x] Weighted selection statistical bounds tests.
- [x] Fake-clock start/expiry/flap tests.
- [x] Precedence and composition matrix.
- [x] Protected-object tests.
- [x] Cap clamp/reject tests.
- [x] Concurrent budget reservation and release tests.
- [x] Emergency disable prevents new selections.
- [x] Simulation has no sleeps, cache writes, budget consumption, or state changes.
- [x] Fuzz policy combinations and selector inputs.
- [x] Regression test for every discovered chaos-core bug.

## Documentation updates

- [x] Record exact deterministic algorithm and golden vectors.
- [x] Finalize selector and schedule schema.
- [x] Update compatibility and error docs.
- [x] Add release-note entry for chaos policy foundation.

## Acceptance criteria

- A policy attached to an exact or wildcard record can be matched and produce an explained outcome.
- Safety caps and protected objects cannot be bypassed by policy configuration.
- Deterministic simulation is stable and side-effect free.

## Handoff

Provide a stable action plan passed to CHA-002 and interfaces for resolver, forwarder, cache, and transport phases.
