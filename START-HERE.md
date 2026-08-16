# Start Here

## For an implementation agent

Before changing code:

1. Read `AGENTS.md` completely.
2. Read the architecture, DNS semantics, chaos engine, state model, control-plane parity, security, and testing documents.
3. Read all ADRs that affect the task.
4. Select one task file from `tasks/` whose dependencies are complete.
5. Create or update tests before declaring the task complete.
6. Update all affected documentation in the same change.
7. Run every required local verification target.

Do not implement REST, MCP, DNS behavior, configuration behavior, or chaos behavior directly from a task summary when a normative design document exists. The design document is the source of truth. If the implementation requires changing an invariant, write an ADR and update the normative documentation first.

## For a coordinating agent

Use `tasks/00-program-board.md` and `tasks/parallelization-plan.md` to allocate work. Parallel work is safe only when package ownership and schema ownership do not overlap. Integration changes to shared domain types, generated schemas, or the capability registry must be serialized or coordinated explicitly.

## Definition of done

A task is not done until:

- The implementation is complete and reviewable.
- Unit and regression tests cover all changed behavior.
- Protocol changes have integration and compatibility tests.
- Configuration changes have positive and negative validation tests.
- REST and MCP parity checks pass.
- Race, fuzz, lint, generation, documentation, and relevant end-to-end checks pass.
- All affected documentation is current.
- No CI check is ignored, bypassed, or marked optional to get a merge.
- User-visible and operator-visible changes are recorded for release notes.

GA-001 (1.0.0-rc.1 candidate) is complete on the program board. Do not create a git tag from an agent change. Evidence: `docs/releases/acceptance-evidence.md`. Notes: `docs/releases/v1.0.0-rc.1.md`. Residual: `docs/known-limitations.md`.
