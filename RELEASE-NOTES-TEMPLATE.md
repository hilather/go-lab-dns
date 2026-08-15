# LabDNS VERSION Release Notes

Release date: YYYY-MM-DD
Previous release: PREVIOUS_VERSION
Application version: VERSION
Configuration versions: LIST
REST versions: LIST
MCP protocol versions: LIST
Container digest: DIGEST

## Highlights

Summarize the most important outcomes, not individual commits.

## Added

List every new user-visible or operator-visible capability.

## Changed

List every behavioral, default, performance, operational, or policy difference.

## Fixed

List correctness fixes, including DNS behavior whose observable output changed.

## Removed or deprecated

List removals, deprecations, replacement paths, and earliest removal versions.

## Security

List authentication, authorization, validation, hardening, dependency, and vulnerability changes.

## DNS behavior

Describe changes to exact records, wildcards, authoritative/overlay behavior, negative answers, forwarding, cache, flags, transports, record types, and interoperability.

## Chaos behavior

Describe every new or changed selector, action, phase, default, cap, protected object, expiry rule, deterministic algorithm, and emergency control.

## REST API

Describe endpoint, schema, default, error, pagination, authorization, and compatibility differences.

## MCP API and protocol compatibility

Describe supported MCP protocol versions, SDK changes, tools, resources, schemas, errors, transport behavior, and migration requirements.

## Configuration and schema

Describe fields, defaults, validation, normalization, migrations, and canonical export differences.

## Deployment and operations

Describe images, ports, flags, environment variables, paths, health, resource guidance, deployment templates, runbooks, and rollback differences.

## Observability

Describe metrics, labels, logs, traces, health semantics, audit schemas, and dashboard changes.

## Compatibility and migration

Provide exact upgrade steps, breaking changes, compatibility windows, and rollback instructions.

## Known limitations

List unresolved limitations and safe workarounds.

## Complete functionality-difference review

Confirm that the generated differences below were reviewed and represented above:

- [ ] OpenAPI
- [ ] MCP capability manifest and tool/resource schemas
- [ ] Configuration schema and defaults
- [ ] CLI flags and environment variables
- [ ] DNS record and chaos action support
- [ ] Error code catalog
- [ ] Metrics and labels
- [ ] Deployment files and image contents
- [ ] Dependencies and SBOM

## CI and release evidence

- [ ] All required CI passed on the exact tag commit.
- [ ] No required check was bypassed.
- [ ] Any CI failure encountered was fixed and hardened.
- [ ] Container digest maps to the tag commit.
- [ ] Security scans were reviewed.
- [ ] Upgrade and rollback were tested.
