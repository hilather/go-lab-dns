# REL-001: CI, Documentation, and Release Automation

Status: done
Recommended owner: Release engineering agent
Dependencies: FND-001 and stable generation commands; may evolve throughout project
Exclusive ownership: release workflows, schema diff tooling, documentation gates

## Goal

Make quality, documentation, compatibility, and complete release differences mechanically enforced.

## Work items

- [x] Make all required CI jobs real and required.
- [x] Add generated-file cleanliness checks.
- [x] Add config, OpenAPI, MCP manifest, CLI, metrics, error catalog, defaults, and chaos-action diff tools.
- [x] Add documentation metadata, link, lint, and example tests.
- [x] Add changelog-entry check for externally observable pull requests.
- [x] Add release-note template with all required categories.
- [x] Add tag gate that refuses release with missing sections or undocumented diffs.
- [x] Add signed tag/image and SBOM/provenance steps where platform support exists.
- [x] Add exact-commit and image-digest verification.
- [x] Retain CI artifacts needed to diagnose failures: test reports, fuzz seeds, race logs, packet captures where safe, schemas, and container logs.
- [x] Add flake tracking and a policy that forbids broad retries.
- [x] Document a CI failure hardening postmortem template.

## Required tests

- [x] CI fails when generated files are stale.
- [x] CI fails on broken documentation link or invalid example.
- [x] CI fails when REST or MCP parity is missing.
- [x] Tag workflow fails with incomplete release notes.
- [x] Tag workflow reports all public-surface differences from previous tag.
- [x] Release cannot proceed after a failed required job.
- [x] Retry policy test rejects unbounded/broad retries where enforceable.
- [x] Release artifacts map to the exact tag commit.
- [x] Regression test for every pipeline defect.

## Documentation updates

- [x] Finalize release engineering, documentation governance, compatibility, and contributing docs.
- [x] Publish local equivalents for every CI job.
- [x] Add release operator checklist.
- [x] Add release-note entry for pipeline changes when user/operator behavior is affected.

## Acceptance criteria

- All CI is required and green on the release candidate.
- A release note contains every functionality difference between tags, not merely commits.
- Documentation freshness is enforced.
- A simulated CI failure is fixed and hardened, demonstrating the policy.

## Handoff

- **Release command:** land a green commit, write `docs/releases/vX.Y.Z.md`, run `make release-diff FROM=<prev> TO=HEAD NOTES=docs/releases/vX.Y.Z.md`, annotated-tag the exact SHA, push the tag, wait for `tag-gate`.
- **Required permissions:** CI `contents: read`. Release `contents: read`, `actions: read`, `checks: read`. Registry publish is operator-side.
- **Artifacts:** `release-diff.txt` (30 days), failing unit/race/fuzz/parity/config-compat logs (14 days), generated catalogs, curated notes.
- **Diff report format:** text lines `id status path` plus optional JSON `{from,to,surfaces[],notes}`.
- **Rollback:** pin the previous image digest; do not rewrite history; next patch tag on a new green commit. See [docs/14-release-engineering.md](https://github.com/hilather/go-lab-dns/blob/main/docs/14-release-engineering.md).
