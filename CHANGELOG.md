# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- Initial architecture and implementation plan.
- Repository foundation: Apache-2.0 license, Go 1.26 module `github.com/hilather/go-lab-dns`, package tree, Makefile, and fail-closed CI skeleton.

### Changed

- None.

### Fixed

- None.

### Removed or deprecated

- None.

### Security

- None.

### DNS behavior

- None.

### Chaos behavior

- None.

### REST API

- None.

### MCP API and protocol compatibility

- None.

### Configuration and schema

- None.

### Deployment and operations

- None.

### Observability

- None.

### Compatibility and migration

- None.

### Known limitations

- DNS, control-plane, chaos, and container image work has not started.
- `make test-integration`, `make test-parity`, `make test-config-compat`, and `make test-container` fail closed until later PRs.
