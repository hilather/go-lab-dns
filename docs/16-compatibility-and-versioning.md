# Compatibility and Versioning

Status: Proposed
Owners: Architecture, Release Engineering
Last reviewed: 2026-08-15 (MCP-001 2026-07-28 pin)
Last reviewed: 2026-08-15 (OBS-001 metrics catalog)
Last reviewed: 2026-08-15 (REL-001 release-diff surfaces)
Last reviewed: 2026-08-15 (PERF-001 interop matrix)
Last reviewed: 2026-08-15 (GA-001 known limitations)
Last reviewed: 2026-08-19 (hashed web/dist is not a release-diff surface)
Last reviewed: 2026-08-19 (Canonical revision-hash change from materialized spec.ui)

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

**1.1.0 compatibility event (Canonical revision hash):** omitted `spec.ui` now materializes `{enabled: true}` into Canonical JSON. Content-addressed `sha256:` revisions therefore change for documents that previously omitted `spec.ui`, even when no other fields changed. `hash-v1` chaos vectors are unaffected. Operators storing `expectedRevision` from 1.0.0-rc.* must re-GET state after upgrade (normal after any Canonical change). `spec.management.allowedOrigins` is additive and omitted-empty.

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

`scripts/release-diff` reports `added` / `removed` / `changed` / `identical` for every public surface between two refs. Human review confirms whether a `changed` file is additive, potentially breaking, or breaking. All differences appear in `docs/releases/<tag>.md`. An unaccounted surface blocks the tag.

Hashed operator-console assets (`web/dist/`, `internal/web/dist/assets/`) are **not** public surfaces. They change every production build. Diff the capability UI map, OpenAPI session operations, `spec.ui` schema, and the metrics `ui.session` event instead. `internal/releasecontract.PublicSurfaces()` is the closed list.

## Testing strategy

- Old config fixture loading.
- Migration round trips.
- REST client compatibility fixtures.
- MCP protocol-version matrix.
- Deterministic chaos golden vectors.
- Metric and error catalog diffs.
- `make test-config-compat` (positive and negative v1alpha1 fixtures).
- Client interop fixtures ([`testdata/interop/cases.json`](https://github.com/hilather/go-lab-dns/blob/main/testdata/interop/cases.json)): exact A, RFC 4592 wildcard, NXDOMAIN, NODATA, empty non-terminal, CNAME chase, TTL, EDE (RFC 8914), UDP TC then TCP.

### Client compatibility matrix (first GA)

| Client | How it is run | Covered behaviors | Known gaps |
|---|---|---|---|
| Raw UDP/TCP (`internal/dnswire` + `internal/interop`) | always | All fixture rows | None |
| `dig` | `TestInteropFixturesDig` when `dig` is on `PATH` | Same rows; `+ignore +notcp` isolates TC | BIND vs Knot `dig` presentation of EDE may differ; fixtures match text + status |
| Go `net.Resolver` (`PreferGo: true`) | always | Exact, wildcard, NXDOMAIN, NODATA AAAA, CNAME follow, TXT via TCP retry | No TTL or EDE API; not glibc/`nscd` |
| glibc / systemd-resolved / Windows | manual lab | Point at the same listener and compare to `cases.json` | Not in CI (would rewrite host resolver config) |

New discovered client differences become additional `cases.json` rows, not one-off test logic.

## Open questions

Resolved for first GA: only `labdns.dev/v1alpha1` is supported. A later `v1beta1`/`v1` must land as a `Migrator` plus old-fixture tests; do not reinterpret v1alpha1 fields in place.

Durable residual (no predecessor tag, unpublished digest, interop gaps, first-GA exclusions): [docs/known-limitations.md](https://github.com/hilather/go-lab-dns/blob/main/docs/known-limitations.md).
