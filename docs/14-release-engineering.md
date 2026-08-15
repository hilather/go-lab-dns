# Release Engineering

Status: Proposed normative release gate
Owners: Release Engineering, All maintainers
Last reviewed: 2026-08-15

## Goals

- Reproducible, reviewable releases.
- Complete functionality differences between tags.
- No release with failing CI.
- Compatibility and migration visibility.
- Hardened pipelines after failures.

## Versioning

Use semantic versioning for the application. Separately version:

- Configuration API.
- REST API base version.
- MCP capability manifest version.
- Supported MCP protocol versions.
- Deterministic chaos algorithm identifiers.

## Required tag gate

Before creating a release tag:

- All required CI checks pass on the exact commit.
- Generated files are current.
- Unit, race, fuzz smoke, integration, parity, config compatibility, container, documentation, and security checks pass.
- The container image is built from the tagged commit.
- API, MCP, config, CLI, metric, and default-value diffs against the previous tag are reviewed.
- Release notes are complete and approved.
- Known limitations are explicit.
- Upgrade and rollback are tested for supported paths.

No administrative bypass is permitted for a required failed check.

## Release notes requirements

Every tag includes all user-visible and operator-visible functionality differences from the previous tag. Organize notes under:

```text
Highlights
Added
Changed
Fixed
Removed or deprecated
Security
DNS behavior
Chaos behavior
REST API
MCP API and protocol compatibility
Configuration and schema
Deployment and operations
Observability
Compatibility and migration
Known limitations
Contributors and acknowledgments
```

A commit or pull-request list may be appended but cannot replace curated notes.

## Automated release-diff inputs

Generate and review differences for:

- OpenAPI document.
- MCP capability manifest and tool schemas.
- Configuration JSON Schema.
- Canonical default configuration.
- CLI help and flags.
- Environment variables.
- Metrics names and labels.
- Error code catalog.
- Supported record types and chaos actions.
- Dependency and SBOM changes.

Any unaccounted functional difference blocks tagging.

## Changelog

Maintain an unreleased section during development. Pull requests with externally observable changes add a concise entry. At release, curate the complete delta, link migration guidance, and freeze it under the version and date.

## CI failure hardening

If release CI fails:

- Stop the release.
- Preserve logs, test reports, packets, seeds, and container artifacts.
- Fix the root cause.
- Add a regression test or pipeline assertion.
- Improve diagnostics and deterministic setup.
- Pin unstable dependencies or images.
- Use a bounded retry only for a demonstrated external transient.
- Document material pipeline changes.

A rerun without a code, test, fixture, or environment explanation is not sufficient evidence of health.

## Artifacts

Recommended release artifacts:

- Signed source tag.
- Multi-architecture container image pinned by digest.
- Checksums.
- SBOM.
- Provenance attestation where supported.
- OpenAPI and MCP capability manifests.
- Configuration schema.
- Example configuration and probes.
- Complete release notes.

## Testing strategy

Test release scripts in CI, including previous-tag discovery, schema diffs, dirty-worktree rejection, missing release-note sections, image-to-tag provenance, and rollback.

## Compatibility implications

The release process is the enforcement point for public-surface compatibility. A release cannot claim compatibility without passing the documented diff and test gates.
