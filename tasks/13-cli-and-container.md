# DEP-001: CLI, Container, and Process Lifecycle

Status: not-started
Recommended owner: Deployment/runtime agent
Dependencies: runnable server and management interfaces
Exclusive ownership: `cmd/labdns`, Dockerfile, process lifecycle, container tests

## Goal

Deliver a production-shaped CLI and hardened non-root container with deterministic startup, validation, health, shutdown, and emergency controls.

## Work items

- [ ] Implement `serve`, `validate`, `canonicalize`, `verify`, `query`, `healthcheck`, and version commands.
- [ ] Implement explicit config paths, listener flags, shutdown deadline, and startup chaos-disable override.
- [ ] Ensure environment variables cannot silently override protected safety caps without documentation.
- [ ] Implement graceful signal handling.
- [ ] Build a multi-stage minimal image.
- [ ] Run as numeric non-root UID with read-only filesystem support.
- [ ] Drop all capabilities and use an unprivileged internal DNS port.
- [ ] Embed or expose build metadata, licenses, supported schemas, and MCP versions.
- [ ] Add multi-architecture build configuration if required.
- [ ] Add container healthcheck command.
- [ ] Produce SBOM/provenance hooks for REL-001.

## Required tests

- [ ] Every CLI command has success and failure tests.
- [ ] Unknown flags and invalid config fail clearly.
- [ ] Container runs non-root and read-only.
- [ ] No capability is required.
- [ ] UDP/TCP host-to-container queries work.
- [ ] Management binding defaults are safe.
- [ ] Restart discards runtime drift.
- [ ] Startup chaos-disable override wins over YAML.
- [ ] Graceful shutdown cancels delayed queries within deadline.
- [ ] Regression test for every CLI/container defect.

## Documentation updates

- [ ] Publish CLI reference and environment variables.
- [ ] Finalize Docker/Compose examples.
- [ ] Update security and operations docs.
- [ ] Add release-note entry for deployment interfaces.

## Acceptance criteria

- Hardened container passes security checks.
- Process lifecycle is predictable under load and chaos.
- Runtime state is ephemeral after recreation.

## Handoff

Provide image path conventions, ports, health commands, and required mounts to GIT-001 and REL-001.
