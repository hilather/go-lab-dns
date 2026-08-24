# Documentation

Operator front door: [README.md](../README.md). Onboarding: [START-HERE.md](../START-HERE.md). Agent rules: [AGENTS.md](../AGENTS.md).

This page is the catalog. Normative design documents win over task summaries.

## Root

| Path | Role |
|---|---|
| [README.md](../README.md) | Product page, quick starts, state APIs |
| [docs/assets/header.jpg](assets/header.jpg) | README banner |
| [docs/assets/social.jpg](assets/social.jpg) | 1280×640 social card |
| [START-HERE.md](../START-HERE.md) | Onboarding and definition of done |
| [AGENTS.md](../AGENTS.md) | Mandatory contributor / agent instructions |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | PR workflow |
| [SECURITY.md](../SECURITY.md) | Vulnerability reporting |
| [CHANGELOG.md](../CHANGELOG.md) | Curated history |
| [MANIFEST.md](../MANIFEST.md) | Pack inventory |
| [LICENSE](../LICENSE) | Apache-2.0 |
| [RELEASE-NOTES-TEMPLATE.md](../RELEASE-NOTES-TEMPLATE.md) | Between-tag notes template |
| [CI-FAILURE-HARDENING-TEMPLATE.md](../CI-FAILURE-HARDENING-TEMPLATE.md) | CI hardening record |

## Architecture

| Path | Topic |
|---|---|
| [01-architecture.md](01-architecture.md) | System model, snapshots, flows |
| [02-dns-semantics.md](02-dns-semantics.md) | Wire, wildcards, CNAME, flags |
| [03-chaos-engine.md](03-chaos-engine.md) | Policies, selectors, effects |
| [04-state-and-configuration.md](04-state-and-configuration.md) | YAML, revisions, plan/apply/export/reset |
| [05-control-plane-and-parity.md](05-control-plane-and-parity.md) | Shared capability registry |
| [22-web-ui.md](22-web-ui.md) | Operator UI bindings and dispositions |
| [implementation-design.md](implementation-design.md) | Implementation design |

## Interfaces

| Path | Topic |
|---|---|
| [06-rest-api.md](06-rest-api.md) | REST `/v1` |
| [07-mcp-api.md](07-mcp-api.md) | MCP tools and protocol pin |
| [22-web-ui.md](22-web-ui.md) | Embedded operator console |
| [09-observability.md](09-observability.md) | Metrics, logs, health |
| [17-error-model.md](17-error-model.md) | Domain errors |

## Security, operations, release

| Path | Topic |
|---|---|
| [08-security-architecture.md](08-security-architecture.md) | Authn/z, trust boundaries |
| [20-threat-model.md](20-threat-model.md) | Threat model |
| [10-testing-strategy.md](10-testing-strategy.md) | Test layers |
| [11-deployment.md](11-deployment.md) | Container and process |
| [12-deployment-repository.md](12-deployment-repository.md) | GitOps layout |
| [13-operations-and-runbooks.md](13-operations-and-runbooks.md) | Runbooks |
| [14-release-engineering.md](14-release-engineering.md) | Tags and surface diffs |
| [15-documentation-governance.md](15-documentation-governance.md) | Docs policy |
| [16-compatibility-and-versioning.md](16-compatibility-and-versioning.md) | Compatibility |
| [18-roadmap-and-non-goals.md](18-roadmap-and-non-goals.md) | Roadmap |
| [19-acceptance-criteria.md](19-acceptance-criteria.md) | GA acceptance |
| [21-standards-and-references.md](21-standards-and-references.md) | RFCs |
| [known-limitations.md](known-limitations.md) | First-GA residual |

## Architecture decisions

| ADR | Decision |
|---|---|
| [0001](adr/0001-use-go.md) | Use Go |
| [0002](adr/0002-purpose-built-hybrid-resolver.md) | Purpose-built hybrid resolver |
| [0003](adr/0003-ephemeral-state-and-gitops.md) | Ephemeral state and GitOps |
| [0004](adr/0004-shared-capability-registry.md) | Shared capability registry |
| [0005](adr/0005-bounded-chaos-engine.md) | Bounded chaos engine |
| [0006](adr/0006-pin-mcp-protocol-versions.md) | Pin MCP protocol versions |
| [0007](adr/0007-defer-unsafe-wire-chaos.md) | Defer unsafe wire chaos |
| [0008](adr/0008-embedded-operator-web-ui.md) | Embedded operator web UI |
| [0009](adr/0009-accept-overlength-desired-state-names.md) | Accept over-length desired-state names |

## Task lists

See [tasks/README.md](../tasks/README.md) and the [program board](../tasks/00-program-board.md).

| Path | Package |
|---|---|
| [00-program-board.md](../tasks/00-program-board.md) | Milestones and status |
| [parallelization-plan.md](../tasks/parallelization-plan.md) | Parallel lanes |
| [reviewer-checklist.md](../tasks/reviewer-checklist.md) | Review bar |
| [agent-task-template.md](../tasks/agent-task-template.md) | Task file template |
| [01-repository-foundation.md](../tasks/01-repository-foundation.md) | FND-001 |
| [02-domain-and-configuration.md](../tasks/02-domain-and-configuration.md) | CFG-001 |
| [03-dns-wire-server.md](../tasks/03-dns-wire-server.md) | DNS-001 |
| [04-local-resolver-and-wildcards.md](../tasks/04-local-resolver-and-wildcards.md) | RES-001 |
| [05-forwarding-and-cache.md](../tasks/05-forwarding-and-cache.md) | FWD-001 |
| [06-snapshot-state-and-mutations.md](../tasks/06-snapshot-state-and-mutations.md) | STA-001 |
| [07-chaos-core.md](../tasks/07-chaos-core.md) | CHA-001 |
| [08-chaos-effects.md](../tasks/08-chaos-effects.md) | CHA-002 |
| [09-rest-control-plane.md](../tasks/09-rest-control-plane.md) | API-001 |
| [10-mcp-control-plane.md](../tasks/10-mcp-control-plane.md) | MCP-001 |
| [11-auth-security-audit.md](../tasks/11-auth-security-audit.md) | SEC-001 |
| [12-observability.md](../tasks/12-observability.md) | OBS-001 |
| [13-cli-and-container.md](../tasks/13-cli-and-container.md) | DEP-001 |
| [14-deployment-examples.md](../tasks/14-deployment-examples.md) | GIT-001 |
| [15-ci-docs-release.md](../tasks/15-ci-docs-release.md) | REL-001 |
| [16-performance-interoperability.md](../tasks/16-performance-interoperability.md) | PERF-001 |
| [17-ga-hardening.md](../tasks/17-ga-hardening.md) | GA-001 |
| [18-web-ui.md](../tasks/18-web-ui.md) | UI-001–UI-004 |

## Releases and contracts

| Path | Role |
|---|---|
| [releases/v1.0.0-rc.1.md](releases/v1.0.0-rc.1.md) | 1.0.0-rc.1 candidate notes (UI-less) |
| [releases/v1.0.0-rc.2.md](releases/v1.0.0-rc.2.md) | 1.0.0-rc.2 notes (MCP mount, toolchain, action pins; UI-less) |
| [releases/v1.1.0.md](releases/v1.1.0.md) | 1.1.0 candidate notes (operator console) |
| [releases/acceptance-evidence.md](releases/acceptance-evidence.md) | Acceptance index |
| [ci-failure-hardening/2026-08-15-cli-help-not-generated.md](ci-failure-hardening/2026-08-15-cli-help-not-generated.md) | CI hardening note |
| [../api/openapi/v1.json](../api/openapi/v1.json) | OpenAPI 3.1 |
| [../api/mcp/v1.json](../api/mcp/v1.json) | MCP manifest |
| [../api/capabilities/v1.json](../api/capabilities/v1.json) | Capability registry |
| [../api/jsonschema/labdns.dev.v1alpha1.json](../api/jsonschema/labdns.dev.v1alpha1.json) | Config schema |
| [../examples/labdns-deploy/README.md](../examples/labdns-deploy/README.md) | GitOps template |
