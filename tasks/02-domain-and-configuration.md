# CFG-001: Domain and Configuration Model

Status: done
Recommended owner: Domain/configuration agent
Dependencies: FND-001
Exclusive ownership: `internal/model`, `internal/config`, source schemas, canonicalization

## Goal

Implement strict, versioned YAML/JSON configuration and canonical domain types for zones, RRsets, forwarding, cache, client groups, and chaos policies.

## Work items

- [x] Define stable IDs and canonical types without importing DNS wire or MCP SDK packages.
- [x] Implement FQDN normalization, owner-to-zone expansion, duration parsing, IP/CIDR parsing, and record-type validation.
- [x] Model authoritative and overlay zones.
- [x] Model common RR types and a validated presentation-format fallback.
- [x] Model forwarding policies, upstream pools, cache bounds, client groups, management settings, and chaos safety/policy structures.
- [x] Implement strict decoding that rejects unknown fields.
- [x] Implement default materialization and deterministic canonical ordering.
- [x] Implement cross-reference validation for IDs and policy references.
- [x] Implement CNAME coexistence and loop validation where statically detectable.
- [x] Reject wildcard DNAME and initial-release wildcard NS.
- [x] Validate chaos action compatibility, safety class, expiry, bounds, alternate addresses, and protected-object rules.
- [x] Generate or maintain the versioned JSON Schema from one source.
- [x] Implement canonical JSON/YAML export and content revision hashing.
- [x] Add config migration interface even if only one version exists.

## Required tests

- [x] Valid complete example parses and canonicalizes.
- [x] Unknown fields fail at every nesting level.
- [x] Duplicate IDs and unresolved references fail.
- [x] Canonicalization is idempotent.
- [x] Export and re-import are semantically equivalent.
- [x] Formatting changes do not change content revision.
- [x] Semantic changes do change revision.
- [x] Invalid RR combinations and names fail with stable field paths.
- [x] Chaos conflicting actions, excessive values, missing expiry, and out-of-range alternate answers fail.
- [x] Old-version fixture migration tests exist.
- [x] YAML/JSON fuzz tests do not panic or allocate without bounds.
- [x] Every validation defect fixed during implementation receives a regression fixture.

## Documentation updates

- [x] Align `docs/04-state-and-configuration.md` with exact field names and defaults.
- [x] Publish generated schema documentation.
- [x] Add valid and invalid examples.
- [x] Update error catalog and compatibility notes.
- [x] Add release-note entry for configuration surfaces.

## Acceptance criteria

- No third-party protocol type appears in public domain structures.
- Strict schema, normalization, canonical export, and revision hashing pass tests.
- The complete sample in the design pack validates.
- All default values and limits are documented.

## Handoff

Provide package APIs for snapshot compilation, API schema use, and DNS wire conversion. Identify any fields intentionally deferred.
