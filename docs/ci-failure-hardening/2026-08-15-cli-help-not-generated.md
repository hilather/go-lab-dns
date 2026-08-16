# CI Failure Hardening Record

Incident/reference: CI-FAILURE-HARDENING / REL-001 simulated pipeline defect
Date: 2026-08-15
Owner: Release engineering
Failed job: generated-file (and the would-be tag-gate public-surface diff)
Commit: REL-001 finalization (this change)

## Failure summary

CLI help is a documented public compatibility surface (`docs/14-release-engineering.md`, `AGENTS.md`). After DEP-001 landed `labdns` flags and `LABDNS_CHAOS_DISABLE`, `make generate` / `make verify-generated` still only treated `testdata/generated/fixture.txt` plus JSON catalogs as generated contracts. A CLI flag or environment-variable change left the worktree “clean” for `generated-file` CI and produced no artifact for `release-diff` to compare.

Simulated by dirtying each generated surface, including `api/cli/help.txt`, and asserting `go run ./scripts/generate -check` fails and names the file. The same gap is reproduced in `scripts/release-diff` by changing only CLI help between two refs and omitting it from curated notes.

Retained evidence: unit logs from `TestVerifyGeneratedFailsWhenEachSurfaceDirty` and `TestRunDiffFailsWhenCLIHelpUndocumented`.

## Classification

- [ ] Product defect
- [ ] Test defect
- [ ] Race or nondeterminism
- [ ] Fixture/environment defect
- [ ] Dependency or supply-chain change
- [x] CI pipeline defect
- [ ] Proven transient external dependency

## Root cause

The generated-file job and its regression test only covered the original FND-001 fixture (and later JSON catalogs). CLI help lived only in `cmd/labdns` `printUsage` and was not a generated contract. `verify-generated` therefore could not fail on a stale or undocumented CLI change. A passing rerun of `generated-file` would not have revealed the gap.

## Immediate fix

- Extract operator-visible help to `internal/clihelp.Text`.
- Write `api/cli/help.txt` from `make generate`.
- Include the file in `internal/releasecontract.PublicSurfaces` (`cli-help`).
- Fail `release-diff --notes` when `cli-help` changes and the notes do not account for it.

## Hardening added

- [x] Regression test
- [x] Better assertion or invariant check
- [ ] Deterministic fixture or fake clock/random source
- [ ] Explicit timeout and cleanup
- [x] Improved failure diagnostics/artifact retention
- [ ] Dependency or image pinning
- [ ] Resource/race/leak check
- [ ] Narrow bounded retry for a proven external transient
- [x] Documentation/runbook update

`TestVerifyGeneratedFailsWhenEachSurfaceDirty` dirties every generated relative path, not only the fixture. `TestHelpMatchesGeneratedArtifact` requires `labdns help` to match `api/cli/help.txt`. `TestRunDiffFailsWhenCLIHelpUndocumented` is the tag-gate counterpart.

## Why recurrence is less likely

Adding a public generated surface without listing it in `releasecontract.GeneratedRels` fails `TestGeneratedRelsCoverPublicSurfaces`. Dirtying any listed file fails `-check`. A CLI-only tag delta without notes fails `release-diff`. The next similar omission (for example a new catalog JSON) has a single SoT and a per-file regression.

## Validation

- `gofmt` on changed Go files
- `go test ./...`
- `make test`
- `make verify-generated`
- `make test-docs`
- `make test-changelog`

## Documentation and release impact

- `docs/14-release-engineering.md` (tag gate, release-diff format, operator checklist)
- `docs/15-documentation-governance.md` (automated checks)
- `CHANGELOG.md` Unreleased / Deployment and operations
- This record

## Review

- [x] Root cause reviewed
- [x] Hardening reviewed
- [ ] All required CI passes
- [x] No check was bypassed or weakened
