# Documentation Governance

Status: Proposed normative maintenance policy
Owners: All maintainers
Last reviewed: 2026-08-15 (REL-001 docs gates)

## Policy

All documentation must be kept up to date. Documentation changes ship in the same pull request as the behavior they describe. Stale documentation blocks completion and release.

## Document classes

- Normative design: architecture, DNS semantics, chaos engine, state, parity, security, compatibility.
- Interface references: REST, MCP, configuration, errors, metrics, CLI.
- Operational: deployment, runbooks, recovery, release.
- Decisions: ADRs.
- Execution: agent tasks and acceptance criteria.

## Required metadata for normative design documents

```text
Status
Owners
Last reviewed
Problem statement
Goals
Non-goals
Invariants where applicable
Detailed design
Failure modes
Security considerations
Observability
Testing strategy
Compatibility implications
Open questions
Related ADRs
```

## Change rules

- Update a normative document before or with an invariant change.
- Add an ADR for an architectural decision that changes an invariant, public semantic, persistence model, trust boundary, protocol baseline, or compatibility policy.
- Update API examples when schemas change.
- Update runbooks when failure handling changes.
- Update task files when dependencies or acceptance criteria change.
- Update `Last reviewed` after substantive review, not mechanical formatting.

## Automated checks

Required CI job `documentation` runs `make test-docs` (`scripts/checkdocs`):

- Required root and workflow files exist (including `.github/workflows/release.yml`).
- Internal-link validation.
- Required metadata (`Status`, `Last reviewed`, and `Owners` unless `Status: Informational`) on numbered `docs/NN-*.md`.
- Example YAML under `examples/` rejects empty files and tab indents.
- Generated reference freshness is `make verify-generated` (job `generated-file`).
- Changelog entry for observable pull requests (`make test-changelog`).
- Release-note completeness and undocumented public-surface diffs (`scripts/release-diff` on tags).

Stale generated OpenAPI, MCP, capabilities, metrics, CLI help, or error catalogs fail `generated-file`. Broken links or missing metadata fail `documentation`.

## Documentation ownership

CODEOWNERS should assign at least:

- DNS semantics owners.
- REST/MCP owners.
- Chaos and security owners.
- Deployment and operations owners.
- Release engineering owners.

## Versioned documentation

Public release documentation must match the tagged version. The main branch may document unreleased behavior only when clearly marked. Configuration and API migrations remain accessible for supported upgrade paths.

## Agent behavior

Agents must not treat documentation as optional cleanup. When implementation reveals that a design is incomplete or wrong, the agent updates the design or proposes an ADR before completing the code task.

## Testing strategy

Documentation checks run in required CI. Broken links, stale generated references, untested examples, or missing release notes fail CI and are fixed rather than bypassed.
