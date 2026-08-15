# LabDNS Architecture and Agent Work Pack

Status: Proposed baseline
Last reviewed: 2026-08-15
Primary implementation language: Go

LabDNS is a container-first DNS override, wildcard, forwarding, and controlled chaos service for laboratories. It is designed for infrastructure-as-a-service deployment, ephemeral runtime state, Git-controlled desired state, and equal control through REST and MCP.

This pack is intended to be copied into a new application repository before implementation begins. It contains:

- Architectural and protocol design documents.
- A bounded DNS chaos model, including per-entry delay and fault behavior.
- REST and MCP parity requirements.
- Configuration, state, security, testing, deployment, and release guidance.
- Architecture Decision Records (ADRs).
- Agent-ready implementation task lists with dependencies and acceptance criteria.
- Root repository instructions for human and AI contributors.

## Recommended reading order

1. [START-HERE.md](START-HERE.md)
2. [AGENTS.md](AGENTS.md)
3. [docs/01-architecture.md](docs/01-architecture.md)
4. [docs/02-dns-semantics.md](docs/02-dns-semantics.md)
5. [docs/03-chaos-engine.md](docs/03-chaos-engine.md)
6. [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md)
7. [docs/05-control-plane-and-parity.md](docs/05-control-plane-and-parity.md)
8. [docs/10-testing-strategy.md](docs/10-testing-strategy.md)
9. [tasks/README.md](tasks/README.md)

## Product summary

LabDNS provides:

- UDP and TCP DNS service.
- Exact DNS overrides and RFC-style wildcard synthesis.
- Authoritative local zones and overlay zones.
- Default and suffix-specific forwarding to configurable upstream pools.
- Common DNS record types plus a validated generic record escape hatch.
- Positive and negative caching.
- Per-record, wildcard, zone, forwarding, upstream, client-group, and global chaos policies.
- Fixed and distributed delay, jitter, intermittent failures, drops, truncation, TTL changes, alternate answers, record omission, cache faults, and upstream faults.
- Strict safety caps, automatic expiry, protected names, and an emergency chaos kill switch.
- REST and MCP interfaces backed by one capability registry and one application layer.
- Immutable compiled runtime snapshots with atomic replacement.
- YAML bootstrap state, deterministic export, drift reporting, and reset-to-bootstrap.
- GitOps-oriented deployment repository guidance.

## Language decision

Go remains the recommended implementation language because it combines mature DNS protocol support, strong concurrency primitives, simple static deployment, race detection, fuzzing, and an official MCP SDK. See [ADR 0001](docs/adr/0001-use-go.md).

## Important scope boundary

LabDNS resolves names to records. Routing many names to different services on the same host still requires an HTTP reverse proxy, ingress controller, TCP proxy, or application-aware gateway that routes by HTTP Host, TLS SNI, port, or another application protocol field.

## License

LabDNS is licensed under [Apache-2.0](https://github.com/hilather/go-lab-dns/blob/main/LICENSE).

## Build and test

LabDNS is the Go module [`github.com/hilather/go-lab-dns`](https://github.com/hilather/go-lab-dns). The required toolchain is **Go 1.26** (`go1.26.x`).

```text
go version   # go1.26.x
make format
make lint
make test
make test-race
make verify-generated
make test-docs
make test-fuzz-smoke
make test-container
make security-scan
```

`make generate` writes `testdata/generated/fixture.txt` from `testdata/generated/source.txt` and `go.mod`, the frozen capability manifest to [`api/capabilities/v1.json`](https://github.com/hilather/go-lab-dns/blob/main/api/capabilities/v1.json), OpenAPI 3.1 to [`api/openapi/v1.json`](https://github.com/hilather/go-lab-dns/blob/main/api/openapi/v1.json), and the MCP manifest to [`api/mcp/v1.json`](https://github.com/hilather/go-lab-dns/blob/main/api/mcp/v1.json). `make verify-generated` fails when any generated file is stale.
`make generate` writes `testdata/generated/fixture.txt` from `testdata/generated/source.txt` and `go.mod`, the frozen capability manifest to [`api/capabilities/v1.json`](https://github.com/hilather/go-lab-dns/blob/main/api/capabilities/v1.json), OpenAPI 3.1 to [`api/openapi/v1.json`](https://github.com/hilather/go-lab-dns/blob/main/api/openapi/v1.json), and the metrics catalog to [`api/metrics/v1alpha1.json`](https://github.com/hilather/go-lab-dns/blob/main/api/metrics/v1alpha1.json). `make verify-generated` fails when any generated file is stale.

`make test-config-compat` runs the v1alpha1 positive and negative configuration fixtures under `testdata/config`.

`make test-container` builds `ghcr.io/hilather/labdns` and checks the non-root / read-only / no-caps contract (requires Docker).

Targets that are not yet implemented fail closed (non-zero exit with an explicit `unimplemented` message) rather than succeeding as no-ops:

```text
make test-integration    # later DNS/control-plane PRs
make test-parity         # REST/MCP capability parity and MCP goldens
make test-container      # PR-16 / DEP-001
make test-parity         # API-001 / MCP-001
```

See [AGENTS.md](https://github.com/hilather/go-lab-dns/blob/main/AGENTS.md) for the full required target list.

### Required CI jobs

The following GitHub Actions jobs are required and have no bypass path:

- `format`
- `lint`
- `unit`
- `race`
- `fuzz-smoke`
- `generated-file`
- `documentation`
- `security-scan`
- `container-test` (builds `ghcr.io/hilather/labdns` and checks the hardened runtime contract)

Local equivalents are the `make` targets above. Do not mark a required check optional to ship a change.
