# STA-001: Snapshot State and Mutations

Status: in-progress (STA-001 slice B: plan/apply/export/reset in `internal/app`; REST/MCP still later)
Recommended owner: Application/state agent
Dependencies: CFG-001 and resolver/forwarder compile interfaces
Exclusive ownership: `internal/snapshot` orchestration helpers, state compiler orchestration, `internal/app` mutation core

## Goal

Implement bootstrap loading, immutable active snapshots, atomic swaps, revisions, idempotent plan/apply, export, drift, reset, and rollback-safe failure handling.

## Work items

- [x] Implement active, bootstrap, and optional previous snapshot references.
- [x] Load one snapshot pointer per DNS request.
- [x] Implement full candidate-state copy, normalize, validate, compile, diff, and impact pipeline.
- [x] Implement dry-run planning and atomic apply.
- [x] Implement expected-revision conflict behavior.
- [x] Implement bounded idempotency-key retention and conflict detection.
- [x] Implement deterministic canonical YAML/JSON export.
- [x] Implement bootstrap-to-runtime operation export.
- [x] Implement reset that rereads and validates before swap.
- [x] Expose generation, revisions, drift, and load time.
- [x] Ensure control-plane failure cannot invalidate active DNS state.
- [x] Define capability handler interfaces used by REST and MCP.

## Required tests

- [x] Concurrent queries see one complete snapshot during swaps.
- [x] Invalid candidate leaves active state unchanged.
- [x] Revision conflict returns current revision.
- [x] Same idempotency key and body returns original result.
- [x] Same key with different body conflicts.
- [x] Reset success, invalid file, missing file, and concurrent query tests.
- [x] Canonical export is deterministic.
- [x] Drift toggles correctly.
- [x] Previous snapshot does not keep unbounded memory.
- [x] Race and leak tests pass.
- [x] Regression test for every mutation/state defect.

## Documentation updates

- [x] Align state and mutation examples with exact schemas.
- [x] Document idempotency retention and reset semantics.
- [x] Update operations runbooks.
- [x] Add release-note entry for runtime state control.

## Acceptance criteria

- DNS continues serving during control-plane failures and state compiles.
- No partial state is observable.
- Plan and apply share the same validation path.
- Reset safely returns to bootstrap state.

## Handoff

`app.Service` is the stable command/query interface. REST/MCP (PR-09/12/13) must call it. Chaos activate/simulate landed in CHA-001.
