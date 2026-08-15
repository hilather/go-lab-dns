# CFG-001: Domain and Configuration Model

Status: not-started
Recommended owner: Domain/configuration agent
Dependencies: FND-001
Exclusive ownership: `internal/model`, `internal/config`, source schemas, canonicalization

## Goal

Implement strict, versioned YAML/JSON configuration and canonical domain types for zones, RRsets, forwarding, cache, client groups, and chaos policies.

## Work items

- [ ] Define stable IDs and canonical types without importing DNS wire or MCP SDK packages.
- [ ] Implement FQDN normalization, owner-to-zone expansion, duration parsing, IP/CIDR parsing, and record-type validation.
- [ ] Model authoritative and overlay zones.
- [ ] Model common RR types and a validated presentation-format fallback.
- [ ] Model forwarding policies, upstream pools, cache bounds, client groups, management settings, and chaos safety/policy structures.
- [ ] Implement strict decoding that rejects unknown fields.
- [ ] Implement default materialization and deterministic canonical ordering.
- [ ] Implement cross-reference validation for IDs and policy references.
- [ ] Implement CNAME coexistence and loop validation where statically detectable.
- [ ] Reject wildcard DNAME and initial-release wildcard NS.
- [ ] Validate chaos action compatibility, safety class, expiry, bounds, alternate addresses, and protected-object rules.
- [ ] Generate or maintain the versioned JSON Schema from one source.
- [ ] Implement canonical JSON/YAML export and content revision hashing.
- [ ] Add config migration interface even if only one version exists.

## Required tests

- [ ] Valid complete example parses and canonicalizes.
- [ ] Unknown fields fail at every nesting level.
- [ ] Duplicate IDs and unresolved references fail.
- [ ] Canonicalization is idempotent.
- [ ] Export and re-import are semantically equivalent.
- [ ] Formatting changes do not change content revision.
- [ ] Semantic changes do change revision.
- [ ] Invalid RR combinations and names fail with stable field paths.
- [ ] Chaos conflicting actions, excessive values, missing expiry, and out-of-range alternate answers fail.
- [ ] Old-version fixture migration tests exist.
- [ ] YAML/JSON fuzz tests do not panic or allocate without bounds.
- [ ] Every validation defect fixed during implementation receives a regression fixture.

## Documentation updates

- [ ] Align `docs/04-state-and-configuration.md` with exact field names and defaults.
- [ ] Publish generated schema documentation.
- [ ] Add valid and invalid examples.
- [ ] Update error catalog and compatibility notes.
- [ ] Add release-note entry for configuration surfaces.

## Acceptance criteria

- No third-party protocol type appears in public domain structures.
- Strict schema, normalization, canonical export, and revision hashing pass tests.
- The complete sample in the design pack validates.
- All default values and limits are documented.

## Handoff

Provide package APIs for snapshot compilation, API schema use, and DNS wire conversion. Identify any fields intentionally deferred.
