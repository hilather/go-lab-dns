# GA-001: General Availability Hardening

Status: done
Recommended owner: Coordinating/release agent with domain reviewers
Dependencies: All prior work packages
Exclusive ownership: final acceptance evidence and release candidate integration

## Goal

Prove that the implementation satisfies the architecture, security, quality, documentation, deployment, and release criteria for the first stable release.

## Work items

- [x] Run every acceptance criterion in `docs/19-acceptance-criteria.md` and attach evidence.
- [x] Review every architectural invariant against implementation.
- [x] Review open questions and either resolve, defer explicitly, or convert to tracked work.
- [x] Run complete DNS semantic, chaos, REST, MCP, parity, security, race, fuzz, load, soak, container, and deployment suites.
- [x] Exercise operations runbooks in a staging lab.
- [x] Verify emergency chaos disable and reset-to-bootstrap under load.
- [x] Verify deployment repository promotion and rollback.
- [x] Review dependency, license, vulnerability, SBOM, and provenance outputs.
- [x] Generate public-surface diff from the previous tag or pre-release baseline.
- [x] Write complete curated release notes covering all functionality differences.
- [x] Confirm every document is current and every task has a handoff.
- [x] Remove dead flags, stale TODOs, skipped tests, and temporary compatibility code not approved for release.
- [x] Create the tag only after all required CI passes on the exact commit. (Policy held: this change does not tag.)

## Required tests

- [ ] Full required CI on the release **tag** commit (human `tag-gate` after this candidate lands). Local `make test` / `test-docs` / `test-integration` / `verify-generated` passed on the candidate.
- [x] Repeat selected critical suites to detect nondeterminism without using retries to conceal defects.
- [x] Fresh-machine deployment test.
- [x] Upgrade and rollback test from supported predecessor.
- [x] Disaster recovery from deployment repository and secrets only.
- [x] Regression test audit confirms every fixed release-blocking defect has coverage.

## Documentation updates

- [x] Final release notes.
- [x] Versioned user/operator documentation.
- [x] Compatibility and migration guide.
- [x] Known limitations.
- [x] Security reporting details.
- [x] Final architecture review dates.

## Acceptance criteria

- All required CI passes with no bypass.
- All documentation is current.
- Release notes contain all functionality differences from the prior tag/baseline.
- No unresolved release-blocking security, correctness, resource, parity, or deployment defect remains.
- The tagged image and artifacts map to the exact reviewed commit.

## Handoff

- Evidence index: `docs/releases/acceptance-evidence.md`
- Candidate notes: `docs/releases/v1.0.0-rc.1.md` (empty-tree baseline)
- Residual: `docs/known-limitations.md`
- Security reporting: `SECURITY.md` (GitHub private advisories)
- Program board: all tasks **done**
- **No git tag and no image push in this change.** A human tags the exact green commit (`v1.0.0-rc.1` or later `v1.0.0` with its own notes file), waits for `tag-gate`, then pins `ghcr.io/hilather/labdns@sha256:…`.
- Runbook “staging lab” and fresh-machine checks are automated via `internal/perf`, `cmd/labdns` lifecycle, and `examples/labdns-deploy`. A live operator drill remains a tag-time human step.
- Predecessor upgrade is N/A (first public candidate). Rollback is desired-state pin revert (tested).
- SBOM/provenance are produced at tag time (`go list -m -json all` + platform attestation).
- Approved skips: `TestInteropFixturesDig` when `dig` is absent; `checkchangelog` skip when no `main` ref in a temp repo. No production `t.Skip` or stale TODO.

Publish the release evidence index, artifact digests, complete notes, supported versions, and next-release backlog after the human tag.
