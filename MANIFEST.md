# Pack Manifest

## Root guidance

- `README.md`: operator-facing product page, YAML/REST/MCP quick starts, and documentation map.
- `docs/assets/header.jpg`: README banner (LabDNS wordmark over the lab bench).
- `docs/assets/social.jpg`: 1280×640 social / Open Graph card.
- `START-HERE.md`: onboarding, five-minute path, and definition of done.
- `docs/README.md`: full documentation catalog (architecture, ADRs, tasks, contracts).
- `AGENTS.md`: mandatory repository instructions.
- `CONTRIBUTING.md`: contribution workflow.
- `SECURITY.md`: top-level security policy (GitHub private advisories).
- `CHANGELOG.md`: curated unreleased and release history.
- `RELEASE-NOTES-TEMPLATE.md`: complete between-tag functionality-difference template.
- `docs/known-limitations.md`: honest first-GA residual (UI no longer a 1.1 non-goal).
- `docs/releases/v1.0.0-rc.1.md`: curated 1.0.0-rc.1 candidate notes (UI-less; do not rewrite).
- `docs/releases/v1.0.0-rc.2.md`: curated 1.0.0-rc.2 notes (MCP mount, Go 1.26.6, action SHA pins; UI-less).
- `docs/releases/v1.1.0.md`: curated 1.1.0 candidate notes (operator console). Unreleased changelog until tag.
- `docs/releases/acceptance-evidence.md`: docs/19 criterion → test/command index (rc.1 plus 1.1.0 console appendix).
- `CI-FAILURE-HARDENING-TEMPLATE.md`: root-cause and pipeline-hardening record.

## Design documents

- `docs/01-architecture.md`
- `docs/02-dns-semantics.md`
- `docs/03-chaos-engine.md`
- `docs/04-state-and-configuration.md`
- `docs/05-control-plane-and-parity.md`
- `docs/06-rest-api.md`
- `docs/07-mcp-api.md`
- `docs/08-security-architecture.md`
- `docs/09-observability.md`
- `docs/10-testing-strategy.md`
- `docs/11-deployment.md`
- `docs/12-deployment-repository.md` (template: `examples/labdns-deploy`)
- `docs/13-operations-and-runbooks.md`
- `docs/14-release-engineering.md`
- `docs/15-documentation-governance.md`
- `docs/16-compatibility-and-versioning.md`
- `docs/17-error-model.md`
- `docs/18-roadmap-and-non-goals.md`
- `docs/19-acceptance-criteria.md`
- `docs/20-threat-model.md`
- `docs/21-standards-and-references.md`
- `docs/22-web-ui.md`

## Architecture decisions

- `docs/adr/0001-use-go.md`
- `docs/adr/0002-purpose-built-hybrid-resolver.md`
- `docs/adr/0003-ephemeral-state-and-gitops.md`
- `docs/adr/0004-shared-capability-registry.md`
- `docs/adr/0005-bounded-chaos-engine.md`
- `docs/adr/0006-pin-mcp-protocol-versions.md`
- `docs/adr/0007-defer-unsafe-wire-chaos.md`
- `docs/adr/0008-embedded-operator-web-ui.md`

## Generated contracts

- `api/capabilities/v1.json`
- `api/openapi/v1.json`
- `api/mcp/v1.json`
- `api/metrics/v1alpha1.json`
- `api/cli/help.txt`
- `api/errors/v1.json`
- `api/jsonschema/labdns.dev.v1alpha1.json` (source schema; compared by release-diff)
- `api/chaos/effects.json` (source catalog; compared by release-diff)

## Agent task plans

See `tasks/README.md` and `tasks/00-program-board.md` for the ordered implementation plan. Operator console: `tasks/18-web-ui.md` (UI-001–UI-004, done).
