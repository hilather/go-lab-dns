# Start here

LabDNS is a container-first laboratory DNS: exact overrides, RFC-style wildcards, suffix forwarding, and bounded chaos. Desired state is YAML. Runtime state is an immutable snapshot you can plan, apply, export, and reset through REST and MCP.

If you want to run it, stay on this page, then follow the [README quick start](README.md#quick-start). If you want to change it, read [AGENTS.md](AGENTS.md) before touching code.

## Five-minute path

1. Install **Go 1.26** and clone this repository.
2. Copy [testdata/container/config.yaml](testdata/container/config.yaml) to `lab.yaml`.
3. `go build -o labdns ./cmd/labdns`
4. `./labdns validate --config lab.yaml`
5. `./labdns serve --config lab.yaml`
6. `./labdns query --name ns1.lab.example.net --server 127.0.0.1:5353`
7. `curl -sS http://127.0.0.1:8080/v1/state`

YAML field rules, revisions, and the plan/apply/export/reset contract live in [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md). REST and MCP twins are in [docs/06-rest-api.md](docs/06-rest-api.md) and [docs/07-mcp-api.md](docs/07-mcp-api.md).

## What to read next

| If you are… | Read |
|---|---|
| Running a lab | [README.md](README.md), [docs/11-deployment.md](docs/11-deployment.md), [examples/labdns-deploy](examples/labdns-deploy/README.md) |
| Writing YAML | [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md), [api/jsonschema/labdns.dev.v1alpha1.json](api/jsonschema/labdns.dev.v1alpha1.json) |
| Breaking DNS on purpose | [docs/03-chaos-engine.md](docs/03-chaos-engine.md), [api/chaos/effects.json](api/chaos/effects.json) |
| Wiring an agent | [docs/05-control-plane-and-parity.md](docs/05-control-plane-and-parity.md), [docs/07-mcp-api.md](docs/07-mcp-api.md) |
| Changing behavior | [AGENTS.md](AGENTS.md), then the normative doc for that area |
| Reviewing the 1.0.0-rc.1 candidate | [docs/releases/v1.0.0-rc.1.md](docs/releases/v1.0.0-rc.1.md), [docs/releases/acceptance-evidence.md](docs/releases/acceptance-evidence.md), [docs/known-limitations.md](docs/known-limitations.md) |

The full catalog — architecture, ADRs, task lists, generated contracts — is in [docs/README.md](docs/README.md) and linked from the [README documentation map](README.md#documentation).

## For contributors and agents

Before changing code:

1. Read [AGENTS.md](AGENTS.md) completely.
2. Read architecture, DNS semantics, chaos, state, control-plane parity, security, and testing: [docs/01-architecture.md](docs/01-architecture.md), [docs/02-dns-semantics.md](docs/02-dns-semantics.md), [docs/03-chaos-engine.md](docs/03-chaos-engine.md), [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md), [docs/05-control-plane-and-parity.md](docs/05-control-plane-and-parity.md), [docs/08-security-architecture.md](docs/08-security-architecture.md), [docs/10-testing-strategy.md](docs/10-testing-strategy.md).
3. Read every ADR that affects the task (`docs/adr/`).
4. Take one file from [tasks/](tasks/README.md) whose dependencies are complete. The board is [tasks/00-program-board.md](tasks/00-program-board.md).
5. Add or update tests before declaring the task done.
6. Update every affected document in the same change.
7. Run every required local verification target (`make test`, `make test-docs`, `make test-changelog`, and the rest listed in [AGENTS.md](AGENTS.md)).

Do not implement REST, MCP, DNS, configuration, or chaos from a task summary when a normative design document exists. The design document is the source of truth. If an invariant must change, write an ADR and update the normative documentation first.

Coordinators allocate work with [tasks/00-program-board.md](tasks/00-program-board.md) and [tasks/parallelization-plan.md](tasks/parallelization-plan.md). Parallel work is safe only when package ownership and schema ownership do not overlap. Integration changes to shared domain types, generated schemas, or the capability registry must be serialized.

### Definition of done

A task is not done until:

- The implementation is complete and reviewable.
- Unit and regression tests cover all changed behavior.
- Protocol changes have integration and compatibility tests.
- Configuration changes have positive and negative validation tests.
- REST and MCP parity checks pass.
- Race, fuzz, lint, generation, documentation, and relevant end-to-end checks pass.
- All affected documentation is current.
- No CI check is ignored, bypassed, or marked optional to get a merge.
- User-visible and operator-visible changes are recorded in [CHANGELOG.md](CHANGELOG.md).

GA-001 (1.0.0-rc.1 candidate) is complete on the program board. Do not create a git tag from an agent change. Evidence: [docs/releases/acceptance-evidence.md](docs/releases/acceptance-evidence.md). Notes: [docs/releases/v1.0.0-rc.1.md](docs/releases/v1.0.0-rc.1.md). Residual: [docs/known-limitations.md](docs/known-limitations.md).
