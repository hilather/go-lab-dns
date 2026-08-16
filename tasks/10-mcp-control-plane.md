# MCP-001: MCP Control Plane

Status: done
Recommended owner: MCP agent
Dependencies: STA-001, stable capability registry, API-001 parity fixtures may proceed in parallel after contract freeze
Exclusive ownership: `internal/control/mcp`, MCP manifests and conformance fixtures

## Goal

Expose the same application capabilities through the pinned MCP protocol using the official Go SDK, with stateless request handling, structured results, and REST parity.

## Work items

- [x] Pin the official Go SDK version and record supported MCP protocol versions.
- [x] Implement Streamable HTTP at `/mcp` for the pinned baseline.
- [x] Validate Origin, protocol version, request metadata, content types, and body limits.
- [x] Register typed tools from or against the capability registry.
- [x] Register read-only resources mirroring REST representations.
- [x] Add optional safe prompts without introducing new capabilities.
- [x] Implement structured result and error mapping.
- [x] Implement cancellation and request deadlines.
- [x] Implement optional stdio adapter if included in scope; keep stdout protocol-clean.
- [x] Generate an MCP capability manifest for release diffs.
- [x] Add subscriptions only if a concrete supported use case and protocol test exists; otherwise defer.

## Required tests

- [x] Official SDK or protocol conformance tests for the pinned version.
- [x] Streamable HTTP request/response and request-scoped streaming tests.
- [x] Origin validation and DNS rebinding defense tests.
- [x] Protocol-version mismatch tests.
- [x] Statelessness tests showing no reliance on connection history.
- [x] Tool and resource schema tests.
- [x] Cancellation and timeout tests.
- [x] Stdio framing and stderr/stdout tests if shipped.
- [x] MCP half of parity goldens for every capability.
- [x] Authorization hook tests.
- [x] Regression test for every MCP defect.

## Documentation updates

- [x] Record exact supported protocol and SDK versions.
- [x] Publish tool/resource schemas and examples.
- [x] Update compatibility and security docs.
- [x] Add release-note entry for MCP support and protocol differences.

## Acceptance criteria

- Every public mutation has REST parity.
- All MCP results are structured and machine-readable.
- Origin and auth protections pass tests.
- Build metadata accurately reports supported protocol versions.

## Handoff

Provide the capability manifest, protocol compatibility matrix, and parity test report.
