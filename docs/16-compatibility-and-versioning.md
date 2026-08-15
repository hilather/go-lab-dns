# Compatibility and Versioning

Status: Proposed
Owners: Architecture, Release Engineering
Last reviewed: 2026-08-15 (MCP-001 2026-07-28 pin)
Last reviewed: 2026-08-15 (OBS-001 metrics catalog)

## Public compatibility surfaces

- DNS resolution semantics and flags.
- Bootstrap configuration versions and defaults.
- Canonical export.
- REST paths, schemas, operation behavior, and errors.
- MCP tool names, schemas, resources, errors, and supported protocol versions.
- CLI flags, environment variables, ports, and filesystem paths.
- Metrics and audit event schemas.
- Deterministic chaos decision algorithms.
- Deployment-repository probe format.

## Application semantic versioning

- Major: breaking public behavior or removal without compatible path.
- Minor: backward-compatible features and new optional fields.
- Patch: backward-compatible fixes, including correctness fixes whose practical behavior change is documented.

A DNS correctness fix may alter observable output. Release notes must describe it even when classified as a patch.

## Configuration versions

Use `apiVersion` and `kind`. First GA ships **`labdns.dev/v1alpha1` only**. `internal/config.Migrator` is the extension point for a later version; `Migrations()` is empty until then. Unknown `apiVersion` values fail with `unsupported_protocol_version`. Unknown fields fail rather than being silently discarded.

## REST

Use `/v1` for the first stable family. Additive optional fields are preferred. Breaking field meaning or removal requires a new API version or a negotiated compatibility strategy.

## MCP

First GA pins **2026-07-28 only** (ADR 0006). The adapter records the pin in `buildinfo.Protocols.MCP` and [api/mcp/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/mcp/v1.json). Do not automatically claim support for a newly published protocol revision. Adapter updates require conformance and parity tests. Tool schemas have their own capability version metadata when needed.

## Chaos algorithm versions

A deterministic selector includes an algorithm identifier such as `hash-v1`. Changing hash inputs, weighting, or time-bucket mapping creates a new algorithm ID so existing experiments remain reproducible.

`hash-v1` is frozen in [docs/03-chaos-engine.md](https://github.com/hilather/go-lab-dns/blob/main/docs/03-chaos-engine.md) and locked by [testdata/hash-v1/vectors.json](https://github.com/hilather/go-lab-dns/blob/main/testdata/hash-v1/vectors.json). Required vectors: `timeBucket: 1s` at two instants in the same UTC second (identical field 9 and digest) and at the next second (different field 9 and digest). Sub-second buckets remain a new algorithm ID.

## Deprecation

A deprecation includes:

- First deprecated release.
- Replacement.
- Earliest removal release or time.
- Runtime and documentation warning strategy.
- Migration tests.

Security emergencies may require faster removal, with explicit notes.

## Schema diff policy

Release CI classifies changes as additive, potentially breaking, or breaking. Human review confirms semantics. All differences appear in release notes.

## Testing strategy

- Old config fixture loading.
- Migration round trips.
- REST client compatibility fixtures.
- MCP protocol-version matrix.
- Deterministic chaos golden vectors.
- Metric and error catalog diffs.
- `make test-config-compat` (positive and negative v1alpha1 fixtures).

## Open questions

Resolved for first GA: only `labdns.dev/v1alpha1` is supported. A later `v1beta1`/`v1` must land as a `Migrator` plus old-fixture tests; do not reinterpret v1alpha1 fields in place.
