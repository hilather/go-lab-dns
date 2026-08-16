# GA-001: General Availability Hardening

Status: done (1.0.0-rc.1 candidate; tag-gate pending)
Recommended owner: Coordinating/release agent with domain reviewers
Dependencies: All prior work packages
Exclusive ownership: final acceptance evidence and release candidate integration

## Goal

Prove that the implementation satisfies the architecture, security, quality, documentation, deployment, and release criteria for the first stable release.

## Work items

- [x] Run every acceptance criterion in `docs/19-acceptance-criteria.md` and attach evidence.
- [x] Review every architectural invariant against implementation.
- [x] Review open questions and either resolve, defer explicitly, or convert to tracked work.
- [ ] Run complete DNS semantic, chaos, REST, MCP, parity, security, race, fuzz, load, soak, container, and deployment suites. (Local recorded evidence on this SHA: `make test` / `test-docs` / `test-integration` / `verify-generated`. Race, fuzz-smoke, container, and `security-scan` remain for the human tag-gate matrix.)
- [ ] Exercise operations runbooks in a staging lab. (Automated stand-ins only; live operator drill is a tag-time human step.)
- [x] Verify emergency chaos disable and reset-to-bootstrap under load. (`internal/perf.TestEmergencyDisableUnderLoad`; `internal/app.TestResetRestoresBootstrap`.)
- [x] Verify deployment repository promotion and rollback. (Automated: `examples/labdns-deploy.TestRollbackRestoresPriorDesiredState`.)
- [ ] Review dependency, license, vulnerability, SBOM, and provenance outputs. (Apache-2.0 and module graph reviewed in-tree. `govulncheck`, SBOM, and provenance are tag-time.)
- [x] Generate public-surface diff from the previous tag or pre-release baseline. (Empty-tree baseline; 8 surfaces added and accounted.)
- [x] Write complete curated release notes covering all functionality differences.
- [x] Confirm every document is current and every task has a handoff.
- [x] Remove dead flags, stale TODOs, skipped tests, and temporary compatibility code not approved for release.
- [x] Create the tag only after all required CI passes on the exact commit. (Policy held: this change does not tag.)

## Required tests

- [ ] Full required CI on the release **tag** commit (human `tag-gate` after this candidate lands). Local `make test` / `test-docs` / `test-integration` / `verify-generated` passed on the candidate.
- [x] Repeat selected critical suites to detect nondeterminism without using retries to conceal defects. (`go test ./...` then `make test` / `test-integration` on the same SHA.)
- [ ] Fresh-machine deployment test. (Template/scripts covered by `examples/labdns-deploy`; live fresh host is tag-time.)
- [ ] Upgrade and rollback test from supported predecessor. (N/A: no predecessor tag. Pin-revert rollback is tested.)
- [x] Disaster recovery from deployment repository and secrets only. (ADR 0003 + `TestNoSecretsInTree` + reset/recreation.)
- [x] Regression test audit confirms every fixed release-blocking defect has coverage.

## Documentation updates

- [x] Final release notes. (Candidate `docs/releases/v1.0.0-rc.1.md`; a later `v1.0.0` tag needs its own file.)
- [x] Versioned user/operator documentation.
- [x] Compatibility and migration guide.
- [x] Known limitations.
- [x] Security reporting details.
- [x] Final architecture review dates.

## Acceptance criteria

- All required CI passes with no bypass. (No bypass in workflows. Tag-commit GitHub green is unmet until `tag-gate`.)
- All documentation is current.
- Release notes contain all functionality differences from the prior tag/baseline.
- No unresolved release-blocking security, correctness, resource, parity, or deployment defect remains.
- The tagged image and artifacts map to the exact reviewed commit. (Unmet: no tag or image in this change.)

## Handoff

- Evidence index: `docs/releases/acceptance-evidence.md`
- Candidate notes: `docs/releases/v1.0.0-rc.1.md` (empty-tree baseline)
- Residual: `docs/known-limitations.md`
- Security reporting: `SECURITY.md` (GitHub private advisories)
- Program board: all tasks **done** as the 1.0.0-rc.1 *candidate*; M5 tag remains a human step
- **No git tag and no image push in this change.** A human tags the exact green commit (`v1.0.0-rc.1` or later `v1.0.0` with its own notes file), waits for `tag-gate`, then pins `ghcr.io/hilather/labdns@sha256:…`.
- Runbook “staging lab” and fresh-machine checks are automated via `internal/perf`, `cmd/labdns` lifecycle, and `examples/labdns-deploy`. A live operator drill remains a tag-time human step.
- Predecessor upgrade is N/A (first public candidate). Rollback is desired-state pin revert (tested).
- SBOM/provenance are produced at tag time (`go list -m -json all` + platform attestation).
- 24h soak was not run; only the 2s CI soak (and optional 30m) exists.
- Approved skips: `TestInteropFixturesDig` when `dig` is absent; `checkchangelog` skip when no `main` ref in a temp repo. No production `t.Skip` or stale TODO.

Publish the release evidence index, artifact digests, complete notes, supported versions, and next-release backlog after the human tag.
