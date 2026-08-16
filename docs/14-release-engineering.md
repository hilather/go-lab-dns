# Release Engineering

Status: Normative release gate (REL-001)
Owners: Release Engineering, All maintainers
Last reviewed: 2026-08-15 (GA-001 1.0.0-rc.1 candidate)

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

## Required CI (no optional jobs)

Every job in [`.github/workflows/ci.yml`](https://github.com/hilather/go-lab-dns/blob/main/.github/workflows/ci.yml) is required. There is no bypass, skip, `continue-on-error`, or unbounded retry. Local equivalents:

```text
make format
make lint
make test                 # unit
make test-race
make test-fuzz-smoke
make verify-generated
make test-docs
make security-scan
make test-container
make test-changelog
make test-parity
make test-config-compat
```

`make test-integration` runs the short PERF-001 suite (`internal/interop`, `internal/perf`, `benches`; default soak 2s). It is **not** a required GitHub job; local and `test-integration` remain fail-closed with no bypass.

Required GitHub job IDs (SoT: `internal/releasecontract.RequiredCIJobs`):

```text
format lint unit race fuzz-smoke generated-file documentation
security-scan container-test changelog parity config-compat
```

Branch protection must require those names. Do not mark a required check optional to ship.

## Required tag gate

The Release workflow [`.github/workflows/release.yml`](https://github.com/hilather/go-lab-dns/blob/main/.github/workflows/release.yml) job `tag-gate` is the only tag path. Before creating a release tag:

- All required CI checks pass on the **exact** tag commit (`release-diff -require-ci`).
- Generated files are current (`make verify-generated`).
- Worktree is clean.
- Unit, race, fuzz smoke, parity, config compatibility, container, documentation, changelog, and security checks passed on that SHA.
- `docs/releases/<tag>.md` exists, uses every required heading from `RELEASE-NOTES-TEMPLATE.md`, and has no leftover template placeholders.
- `release-diff <previous-tag> HEAD -notes docs/releases/<tag>.md` reports every public-surface difference and refuses undocumented ones.
- Known limitations are explicit.
- Upgrade and rollback were tested for supported paths.

No administrative bypass is permitted for a required failed check. A skipped or cancelled required job fails the gate.

### Operator checklist

1. Land the release-candidate commit on `main` with required CI green. GA-001 attached [docs/releases/acceptance-evidence.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/acceptance-evidence.md) and candidate notes [docs/releases/v1.0.0-rc.1.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.0.0-rc.1.md). **Do not tag from an agent change;** a human tags the exact green SHA.
2. Write `docs/releases/vX.Y.Z.md` from `RELEASE-NOTES-TEMPLATE.md` (or copy `docs/releases/v1.0.0-rc.1.md` for `v1.0.0-rc.1`).
3. Run `make verify-generated` and `make release-diff FROM=<prev> TO=HEAD NOTES=docs/releases/vX.Y.Z.md` locally (`<prev>` is the previous `v*` tag, or the empty-tree SHA from `go run ./scripts/release-diff -print-empty-tree` for the first tag).
4. Freeze `CHANGELOG.md` Unreleased into `## vX.Y.Z — YYYY-MM-DD`.
5. Create an annotated tag on the **exact** green SHA (`git tag -a vX.Y.Z <sha>`). Do not retag a different commit.
6. Push the tag. `tag-gate` must succeed. If it fails, delete the remote tag only after a documented hardening record; do not “force” a green run.
7. Build `ghcr.io/hilather/labdns` from that tag and pin the image **digest** in the deployment repository.
8. Attach OpenAPI, MCP manifest, config schema, capability table, metrics catalog, CLI help, error catalog, and the curated notes as release artifacts.
9. Sign the tag when signing keys are available. Produce an SBOM (`go list -m -json all`) and, where the platform supports it, a provenance attestation.

### Required permissions

| Workflow | Permissions | Why |
|---|---|---|
| CI | `contents: read` | checkout and tests |
| Release / tag-gate | `contents: read`, `actions: read`, `checks: read` | exact-commit check-run query |

Tag creation is a human/operator git write. The gate does not use `contents: write` and does not skip checks.

## Release notes requirements

Every tag includes all user-visible and operator-visible functionality differences from the previous tag. The file **must** be `docs/releases/<tag>.md` (for example `docs/releases/v1.0.0.md`). Organize notes under the headings in `RELEASE-NOTES-TEMPLATE.md`:

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
Complete functionality-difference review
CI and release evidence
```

A commit or pull-request list may be appended but cannot replace curated notes. Template placeholders (`YYYY-MM-DD`, `PREVIOUS_VERSION`, `LabDNS VERSION Release Notes`, …) fail the gate.

## Automated release-diff

`scripts/release-diff` compares two git refs. Public surfaces (SoT: `internal/releasecontract.PublicSurfaces`):

| ID | Path | Generated |
|---|---|---|
| openapi | `api/openapi/v1.json` | yes |
| mcp | `api/mcp/v1.json` | yes |
| config-schema | `api/jsonschema/labdns.dev.v1alpha1.json` | no (source schema) |
| capabilities | `api/capabilities/v1.json` | yes (review token: Capability table; not the MCP box) |
| metrics | `api/metrics/v1alpha1.json` | yes |
| cli-help | `api/cli/help.txt` | yes |
| error-catalog | `api/errors/v1.json` | yes |
| chaos-effects | `api/chaos/effects.json` | no (source catalog) |

CLI help and environment variables are the `cli-help` surface (`labdns help` must match `api/cli/help.txt`). Dependency/SBOM diffs are reviewed from `go list -m all` at tag time and recorded in notes; they are not a generated JSON surface in first GA.

### Commands

```text
go run ./scripts/release-diff -previous-tag
go run ./scripts/release-diff -print-empty-tree
go run ./scripts/release-diff -notes-only -notes docs/releases/vX.Y.Z.md
go run ./scripts/release-diff FROM TO
go run ./scripts/release-diff -notes docs/releases/vX.Y.Z.md FROM TO
go run ./scripts/release-diff -json -notes docs/releases/vX.Y.Z.md FROM TO
go run ./scripts/release-diff -require-ci                 # needs GH_TOKEN, GITHUB_REPOSITORY, GITHUB_SHA
go run ./scripts/release-diff -require-ci -ci-fixture testdata/ci/checks.json
make release-diff FROM=<prev> TO=HEAD NOTES=docs/releases/vX.Y.Z.md
```

A dirty worktree fails the tool (`-allow-dirty` is test-only). Missing previous tag compares against the empty tree (every present surface is `added`).

### Diff report format

Text (default):

```text
release-diff <from> → <to>
  openapi          changed    api/openapi/v1.json
  cli-help         added      api/cli/help.txt
  metrics          identical  api/metrics/v1alpha1.json
N public-surface difference(s) require curated release notes
```

JSON (`-json`): `{from,to,surfaces:[{id,path,title,status,fromSha256,toSha256}],notes?}`. `status` is `identical` | `changed` | `added` | `removed` | `missing-both`.

A changed/added/removed surface is **accounted** when the notes mention its ID, title, or path, or check its **unique** review-box (`- [x] …`). The MCP box does not cover a capability-table delta. Unaccounted diffs, missing headings, leftover placeholders, or unchecked boxes for changed surfaces fail the command (exit 1). The surface table is always written to **stdout** (including on notes failure) so `tee release-diff.txt` retains it. `tag-gate` uploads that file as an artifact (30 days).

Any unaccounted functional difference blocks tagging.

## Changelog

Maintain an unreleased section during development. Pull requests with externally observable changes add a concise entry. CI job `changelog` runs `make test-changelog` (`scripts/checkchangelog`) against `origin/<base>` (or `GITHUB_EVENT_BEFORE` on push). Observable paths include `api/`, `cmd/`, `internal/` (except `*_test.go`), `scripts/`, `.github/workflows/`, `Dockerfile`, `Makefile`, `README.md`, `SECURITY.md`, `AGENTS.md`, and the operator-facing `docs/06`, `07`, `09`, `11`, `13`, `14`, `16`, `17` files.

At release, curate the complete delta, link migration guidance, and freeze it under the version and date.

## CI failure hardening

If release CI fails:

- Stop the release.
- Preserve logs, test reports, packets, seeds, and container artifacts (workflow `upload-artifact` on failure; release-diff report on `always()`).
- Fix the root cause.
- Add a regression test or pipeline assertion.
- Improve diagnostics and deterministic setup.
- Pin unstable dependencies or images.
- Use a bounded retry only for a demonstrated external transient.
- Document material pipeline changes using `CI-FAILURE-HARDENING-TEMPLATE.md` under `docs/ci-failure-hardening/`.

A rerun without a code, test, fixture, or environment explanation is not sufficient evidence of health.

### Flake and retry policy

Broad retries are forbidden. Workflows must not set `continue-on-error: true`, `max-attempts`, or unbounded `retry:`. `scripts/checktargets` fails if those appear. A flake is a product or pipeline defect: fix determinism, do not label-skip or retry-until-green.

Demonstrated hardening record: [docs/ci-failure-hardening/2026-08-15-cli-help-not-generated.md](https://github.com/hilather/go-lab-dns/blob/main/docs/ci-failure-hardening/2026-08-15-cli-help-not-generated.md).

## Artifacts

Recommended release artifacts:

- Signed source tag on the exact green commit.
- Multi-architecture container image `ghcr.io/hilather/labdns` pinned by digest (image built from the tagged commit).
- Checksums.
- SBOM (`go list -m -json all`).
- Provenance attestation where the platform supports it.
- OpenAPI, MCP, capabilities, metrics, CLI help, and error catalogs.
- Configuration schema and chaos-effect catalog.
- Example configuration and probes.
- Complete release notes (`docs/releases/<tag>.md`).

### Exact-commit and image-digest verification

- `tag-gate` records `git rev-parse HEAD` and `EvaluateChecks` rejects a required job whose `head_sha` is not that commit.
- Operators pin the container by digest, not `latest` or a moving tag. Confirm the image label / provenance refers to the same git SHA as the source tag. First GA does not push the image from this workflow (no registry write permission on the gate).

## Rollback

1. Do not delete git history. If the tag must not ship, move consumers back to the previous tag and previous image digest.
2. Restore the previous deployment-repository pin (`image: ghcr.io/hilather/labdns@sha256:…`).
3. Runtime rollback is process restart onto the previous image; LabDNS does not persist runtime mutations (ADR 0003).
4. Record the rollback in `CHANGELOG.md` and the next notes file.
5. If the failed tag was pushed, leave it annotated with the failure unless a signed replacement tag is cut on a new green commit (`vX.Y.Z+rebuild` is not used; bump the patch).

## Testing strategy

Test release scripts in CI, including previous-tag discovery, schema diffs, dirty-worktree rejection, missing release-note sections, undocumented CLI help, required-CI failure, and retry-policy rejection. Image-to-tag provenance is an operator check at publish time.

## Compatibility implications

The release process is the enforcement point for public-surface compatibility. A release cannot claim compatibility without passing the documented diff and test gates.
