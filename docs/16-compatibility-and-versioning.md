# Compatibility and Versioning

Status: Proposed
Owners: Architecture, Release Engineering
Last reviewed: 2026-08-15

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

Use `apiVersion` and `kind`. Parsers support explicitly documented versions. Migration tools convert old canonical state to a newer version. Unknown fields fail rather than being silently discarded.

## REST

Use `/v1` for the first stable family. Additive optional fields are preferred. Breaking field meaning or removal requires a new API version or a negotiated compatibility strategy.

## MCP

Pin supported MCP protocol versions and test each. Do not automatically claim support for a newly published protocol revision. Adapter updates require conformance and parity tests. Tool schemas have their own capability version metadata when needed.

## Chaos algorithm versions

A deterministic selector includes an algorithm identifier such as `hash-v1`. Changing hash inputs, weighting, or time-bucket mapping creates a new algorithm ID so existing experiments remain reproducible.

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

## Open questions

- Number of previous config and MCP protocol versions supported at GA.
