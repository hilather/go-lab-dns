# REL-001: CI, Documentation, and Release Automation

Status: not-started
Recommended owner: Release engineering agent
Dependencies: FND-001 and stable generation commands; may evolve throughout project
Exclusive ownership: release workflows, schema diff tooling, documentation gates

## Goal

Make quality, documentation, compatibility, and complete release differences mechanically enforced.

## Work items

- [ ] Make all required CI jobs real and required.
- [ ] Add generated-file cleanliness checks.
- [ ] Add config, OpenAPI, MCP manifest, CLI, metrics, error catalog, defaults, and chaos-action diff tools.
- [ ] Add documentation metadata, link, lint, and example tests.
- [ ] Add changelog-entry check for externally observable pull requests.
- [ ] Add release-note template with all required categories.
- [ ] Add tag gate that refuses release with missing sections or undocumented diffs.
- [ ] Add signed tag/image and SBOM/provenance steps where platform support exists.
- [ ] Add exact-commit and image-digest verification.
- [ ] Retain CI artifacts needed to diagnose failures: test reports, fuzz seeds, race logs, packet captures where safe, schemas, and container logs.
- [ ] Add flake tracking and a policy that forbids broad retries.
- [ ] Document a CI failure hardening postmortem template.

## Required tests

- [ ] CI fails when generated files are stale.
- [ ] CI fails on broken documentation link or invalid example.
- [ ] CI fails when REST or MCP parity is missing.
- [ ] Tag workflow fails with incomplete release notes.
- [ ] Tag workflow reports all public-surface differences from previous tag.
- [ ] Release cannot proceed after a failed required job.
- [ ] Retry policy test rejects unbounded/broad retries where enforceable.
- [ ] Release artifacts map to the exact tag commit.
- [ ] Regression test for every pipeline defect.

## Documentation updates

- [ ] Finalize release engineering, documentation governance, compatibility, and contributing docs.
- [ ] Publish local equivalents for every CI job.
- [ ] Add release operator checklist.
- [ ] Add release-note entry for pipeline changes when user/operator behavior is affected.

## Acceptance criteria

- All CI is required and green on the release candidate.
- A release note contains every functionality difference between tags, not merely commits.
- Documentation freshness is enforced.
- A simulated CI failure is fixed and hardened, demonstrating the policy.

## Handoff

Provide release command, required permissions, artifacts, diff report format, and rollback procedure.
