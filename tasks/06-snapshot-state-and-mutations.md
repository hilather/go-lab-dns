# STA-001: Snapshot State and Mutations

Status: not-started
Recommended owner: Application/state agent
Dependencies: CFG-001 and resolver/forwarder compile interfaces
Exclusive ownership: `internal/snapshot`, state compiler orchestration, `internal/app` mutation core

## Goal

Implement bootstrap loading, immutable active snapshots, atomic swaps, revisions, idempotent plan/apply, export, drift, reset, and rollback-safe failure handling.

## Work items

- [ ] Implement active, bootstrap, and optional previous snapshot references.
- [ ] Load one snapshot pointer per DNS request.
- [ ] Implement full candidate-state copy, normalize, validate, compile, diff, and impact pipeline.
- [ ] Implement dry-run planning and atomic apply.
- [ ] Implement expected-revision conflict behavior.
- [ ] Implement bounded idempotency-key retention and conflict detection.
- [ ] Implement deterministic canonical YAML/JSON export.
- [ ] Implement bootstrap-to-runtime operation export.
- [ ] Implement reset that rereads and validates before swap.
- [ ] Expose generation, revisions, drift, and load time.
- [ ] Ensure control-plane failure cannot invalidate active DNS state.
- [ ] Define capability handler interfaces used by REST and MCP.

## Required tests

- [ ] Concurrent queries see one complete snapshot during swaps.
- [ ] Invalid candidate leaves active state unchanged.
- [ ] Revision conflict returns current revision.
- [ ] Same idempotency key and body returns original result.
- [ ] Same key with different body conflicts.
- [ ] Reset success, invalid file, missing file, and concurrent query tests.
- [ ] Canonical export is deterministic.
- [ ] Drift toggles correctly.
- [ ] Previous snapshot does not keep unbounded memory.
- [ ] Race and leak tests pass.
- [ ] Regression test for every mutation/state defect.

## Documentation updates

- [ ] Align state and mutation examples with exact schemas.
- [ ] Document idempotency retention and reset semantics.
- [ ] Update operations runbooks.
- [ ] Add release-note entry for runtime state control.

## Acceptance criteria

- DNS continues serving during control-plane failures and state compiles.
- No partial state is observable.
- Plan and apply share the same validation path.
- Reset safely returns to bootstrap state.

## Handoff

Provide stable application command/query interfaces and mutation result structures for the capability registry.
