# API-001: REST Control Plane

Status: not-started
Recommended owner: REST/API agent
Dependencies: STA-001 and stable capability registry contract
Exclusive ownership: `internal/control/rest`, OpenAPI binding/generation

## Goal

Expose the shared application capabilities through a versioned REST API with strict schemas, stable errors, and no duplicated business logic.

## Work items

- [ ] Register routes from or against the capability registry.
- [ ] Implement health, version, capabilities, state, validate, plan, apply, export, reset, zone/record reads, resolve/explain, forwarding/cache, chaos, and audit endpoints.
- [ ] Implement JSON and canonical YAML representations where documented.
- [ ] Implement `application/problem+json` mapping from domain errors.
- [ ] Implement opaque pagination and typed filters.
- [ ] Implement request size, deadline, rate, and content-type limits.
- [ ] Implement idempotency and revision headers/body mapping.
- [ ] Generate or verify OpenAPI from the source registry/schema.
- [ ] Add request IDs and trace propagation.
- [ ] Keep auth as shared middleware/hook compatible with SEC-001.

## Required tests

- [ ] Every operation validates against OpenAPI.
- [ ] All domain error codes map correctly.
- [ ] Body, timeout, pagination, and rate-limit tests.
- [ ] Revision and idempotency tests.
- [ ] Resolve/explain and chaos simulation tests.
- [ ] Emergency-disable authorization hook tests.
- [ ] No handler contains domain mutation logic regression test or architecture check.
- [ ] REST half of parity goldens for every capability.
- [ ] Regression test for every endpoint bug.

## Documentation updates

- [ ] Publish exact endpoint and schema reference.
- [ ] Update examples and error catalog.
- [ ] Update security deployment guidance for management listener.
- [ ] Add release-note entry for REST capabilities.

## Acceptance criteria

- OpenAPI is complete and generated-file verification passes.
- Every public route maps to one shared capability.
- All required API tests pass.
- No permissive CORS or unauthenticated remote default is introduced.

## Handoff

Provide operation IDs and normalized transport envelopes for MCP parity testing.
