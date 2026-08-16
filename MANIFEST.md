# Pack Manifest

## Root guidance

- `README.md`: project summary and reading order.
- `START-HERE.md`: onboarding and definition of done.
- `AGENTS.md`: mandatory repository instructions.
- `CONTRIBUTING.md`: contribution workflow.
- `SECURITY.md`: top-level security policy.
- `CHANGELOG.md`: curated unreleased and release history.
- `RELEASE-NOTES-TEMPLATE.md`: complete between-tag functionality-difference template.
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

## Architecture decisions

- `docs/adr/0001-use-go.md`
- `docs/adr/0002-purpose-built-hybrid-resolver.md`
- `docs/adr/0003-ephemeral-state-and-gitops.md`
- `docs/adr/0004-shared-capability-registry.md`
- `docs/adr/0005-bounded-chaos-engine.md`
- `docs/adr/0006-pin-mcp-protocol-versions.md`
- `docs/adr/0007-defer-unsafe-wire-chaos.md`

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

See `tasks/README.md` and `tasks/00-program-board.md` for the ordered implementation plan.
