# MCP-001: MCP Control Plane

Status: not-started
Recommended owner: MCP agent
Dependencies: STA-001, stable capability registry, API-001 parity fixtures may proceed in parallel after contract freeze
Exclusive ownership: `internal/control/mcp`, MCP manifests and conformance fixtures

## Goal

Expose the same application capabilities through the pinned MCP protocol using the official Go SDK, with stateless request handling, structured results, and REST parity.

## Work items

- [ ] Pin the official Go SDK version and record supported MCP protocol versions.
- [ ] Implement Streamable HTTP at `/mcp` for the pinned baseline.
- [ ] Validate Origin, protocol version, request metadata, content types, and body limits.
- [ ] Register typed tools from or against the capability registry.
- [ ] Register read-only resources mirroring REST representations.
- [ ] Add optional safe prompts without introducing new capabilities.
- [ ] Implement structured result and error mapping.
- [ ] Implement cancellation and request deadlines.
- [ ] Implement optional stdio adapter if included in scope; keep stdout protocol-clean.
- [ ] Generate an MCP capability manifest for release diffs.
- [ ] Add subscriptions only if a concrete supported use case and protocol test exists; otherwise defer.

## Required tests

- [ ] Official SDK or protocol conformance tests for the pinned version.
- [ ] Streamable HTTP request/response and request-scoped streaming tests.
- [ ] Origin validation and DNS rebinding defense tests.
- [ ] Protocol-version mismatch tests.
- [ ] Statelessness tests showing no reliance on connection history.
- [ ] Tool and resource schema tests.
- [ ] Cancellation and timeout tests.
- [ ] Stdio framing and stderr/stdout tests if shipped.
- [ ] MCP half of parity goldens for every capability.
- [ ] Authorization hook tests.
- [ ] Regression test for every MCP defect.

## Documentation updates

- [ ] Record exact supported protocol and SDK versions.
- [ ] Publish tool/resource schemas and examples.
- [ ] Update compatibility and security docs.
- [ ] Add release-note entry for MCP support and protocol differences.

## Acceptance criteria

- Every public mutation has REST parity.
- All MCP results are structured and machine-readable.
- Origin and auth protections pass tests.
- Build metadata accurately reports supported protocol versions.

## Handoff

Provide the capability manifest, protocol compatibility matrix, and parity test report.
