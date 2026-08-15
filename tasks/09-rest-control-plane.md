# API-001: REST Control Plane

Status: implemented
Recommended owner: REST/API agent
Dependencies: STA-001 and stable capability registry contract
Exclusive ownership: `internal/control/rest`, OpenAPI binding/generation

## Goal

Expose the shared application capabilities through a versioned REST API with strict schemas, stable errors, and no duplicated business logic.

## Work items

- [x] Register routes from or against the capability registry.
- [x] Implement health, version, capabilities, state, validate, plan, apply, export, reset, zone/record reads, resolve/explain, forwarding/cache, chaos, and audit endpoints.
- [x] Implement JSON and canonical YAML representations where documented.
- [x] Implement `application/problem+json` mapping from domain errors.
- [x] Implement opaque pagination and typed filters.
- [x] Implement request size, deadline, rate, and content-type limits.
- [x] Implement idempotency and revision headers/body mapping.
- [x] Generate or verify OpenAPI from the source registry/schema.
- [x] Add request IDs and trace propagation.
- [x] Keep auth as shared middleware/hook compatible with SEC-001.

## Required tests

- [x] Every operation validates against OpenAPI.
- [x] All domain error codes map correctly.
- [x] Body, timeout, pagination, and rate-limit tests.
- [x] Revision and idempotency tests.
- [x] Resolve/explain and chaos simulation tests.
- [x] Emergency-disable authorization hook tests.
- [x] No handler contains domain mutation logic regression test or architecture check.
- [ ] REST half of parity goldens for every capability (MCP-001 / PR-13).
- [x] Regression test for every endpoint bug.

## Documentation updates

- [x] Publish exact endpoint and schema reference.
- [x] Update examples and error catalog.
- [x] Update security deployment guidance for management listener.
- [x] Add release-note entry for REST capabilities.

## Acceptance criteria

- OpenAPI is complete and generated-file verification passes.
- Every public route maps to one shared capability.
- All required API tests pass.
- No permissive CORS or unauthenticated remote default is introduced.

## Handoff

OpenAPI operation IDs are `{method}_{path}` with `/` and `:` folded to `_` (see [api/openapi/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/openapi/v1.json)). Problem envelopes match `capabilities.ProblemFrom`. MCP-001 should reuse the same `app.Service` methods and domain error mapping.
