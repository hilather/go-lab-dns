# DEP-001: CLI, Container, and Process Lifecycle

Status: done
Recommended owner: Deployment/runtime agent
Dependencies: runnable server and management interfaces
Exclusive ownership: `cmd/labdns`, Dockerfile, process lifecycle, container tests

## Goal

Deliver a production-shaped CLI and hardened non-root container with deterministic startup, validation, health, shutdown, and emergency controls.

## Work items

- [x] Implement `serve`, `validate`, `canonicalize`, `verify`, `query`, `healthcheck`, and version commands.
- [x] Implement explicit config paths, listener flags, shutdown deadline, and startup chaos-disable override.
- [x] Ensure environment variables cannot silently override protected safety caps without documentation.
- [x] Implement graceful signal handling.
- [x] Build a multi-stage minimal image.
- [x] Run as numeric non-root UID with read-only filesystem support.
- [x] Drop all capabilities and use an unprivileged internal DNS port.
- [x] Embed or expose build metadata, licenses, supported schemas, and MCP versions.
- [x] Add multi-architecture build configuration if required.
- [x] Add container healthcheck command.
- [x] Produce SBOM/provenance hooks for REL-001.

## Required tests

- [x] Every CLI command has success and failure tests.
- [x] Unknown flags and invalid config fail clearly.
- [x] Container runs non-root and read-only.
- [x] No capability is required.
- [x] UDP/TCP host-to-container queries work.
- [x] Management binding defaults are safe.
- [x] Restart discards runtime drift.
- [x] Startup chaos-disable override wins over YAML.
- [x] Graceful shutdown cancels delayed queries within deadline.
- [x] Regression test for every CLI/container defect.

## Documentation updates

- [x] Publish CLI reference and environment variables.
- [x] Finalize Docker/Compose examples.
- [x] Update security and operations docs.
- [x] Add release-note entry for deployment interfaces.

## Acceptance criteria

- Hardened container passes security checks.
- Process lifecycle is predictable under load and chaos.
- Runtime state is ephemeral after recreation.

## Handoff

Provide image path conventions, ports, health commands, and required mounts to GIT-001 and REL-001.
