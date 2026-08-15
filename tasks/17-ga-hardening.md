# GA-001: General Availability Hardening

Status: not-started
Recommended owner: Coordinating/release agent with domain reviewers
Dependencies: All prior work packages
Exclusive ownership: final acceptance evidence and release candidate integration

## Goal

Prove that the implementation satisfies the architecture, security, quality, documentation, deployment, and release criteria for the first stable release.

## Work items

- [ ] Run every acceptance criterion in `docs/19-acceptance-criteria.md` and attach evidence.
- [ ] Review every architectural invariant against implementation.
- [ ] Review open questions and either resolve, defer explicitly, or convert to tracked work.
- [ ] Run complete DNS semantic, chaos, REST, MCP, parity, security, race, fuzz, load, soak, container, and deployment suites.
- [ ] Exercise operations runbooks in a staging lab.
- [ ] Verify emergency chaos disable and reset-to-bootstrap under load.
- [ ] Verify deployment repository promotion and rollback.
- [ ] Review dependency, license, vulnerability, SBOM, and provenance outputs.
- [ ] Generate public-surface diff from the previous tag or pre-release baseline.
- [ ] Write complete curated release notes covering all functionality differences.
- [ ] Confirm every document is current and every task has a handoff.
- [ ] Remove dead flags, stale TODOs, skipped tests, and temporary compatibility code not approved for release.
- [ ] Create the tag only after all required CI passes on the exact commit.

## Required tests

- [ ] Full required CI on the release commit.
- [ ] Repeat selected critical suites to detect nondeterminism without using retries to conceal defects.
- [ ] Fresh-machine deployment test.
- [ ] Upgrade and rollback test from supported predecessor.
- [ ] Disaster recovery from deployment repository and secrets only.
- [ ] Regression test audit confirms every fixed release-blocking defect has coverage.

## Documentation updates

- [ ] Final release notes.
- [ ] Versioned user/operator documentation.
- [ ] Compatibility and migration guide.
- [ ] Known limitations.
- [ ] Security reporting details.
- [ ] Final architecture review dates.

## Acceptance criteria

- All required CI passes with no bypass.
- All documentation is current.
- Release notes contain all functionality differences from the prior tag/baseline.
- No unresolved release-blocking security, correctness, resource, parity, or deployment defect remains.
- The tagged image and artifacts map to the exact reviewed commit.

## Handoff

Publish the release evidence index, artifact digests, complete notes, supported versions, and next-release backlog.
