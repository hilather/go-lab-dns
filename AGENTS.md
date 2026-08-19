# Repository Instructions for Agents

These instructions apply to every human or AI agent working in this repository. More specific `AGENTS.md` files may add stricter rules but may not weaken this file.

## Required reading

Before modifying code, read:

1. `docs/01-architecture.md`
2. `docs/02-dns-semantics.md`
3. `docs/03-chaos-engine.md`
4. `docs/04-state-and-configuration.md`
5. `docs/05-control-plane-and-parity.md`
6. `docs/08-security-architecture.md`
7. `docs/10-testing-strategy.md`
8. Every ADR relevant to the area being changed

## Architectural rules

- REST and MCP are adapters. Domain behavior belongs in the shared application, model, resolver, forwarder, cache, chaos, or policy packages.
- REST handlers and MCP handlers must never implement independent business logic.
- Every public capability must be represented in the central capability registry.
- DNS queries must operate against an immutable compiled snapshot.
- State changes must build and validate a complete candidate snapshot before atomically replacing the active snapshot.
- The service must not write to the bootstrap configuration file.
- Do not add a database, journal, hidden volume, or other persistence mechanism without an approved ADR.
- Do not expose third-party DNS library types outside the DNS wire adapter.
- Do not add RFC 2136, AXFR, IXFR, root-hints recursion, DNSSEC signing, or cluster consensus without an approved ADR.
- Chaos behavior must never affect the REST listener, MCP listener, readiness endpoint, liveness endpoint, or emergency chaos-disable path.
- Unsafe malformed-packet generation is not part of the initial product. Adding it requires a separate isolated component and an approved security ADR.

## Tests and regressions

- Every area must have regression tests.
- Every code path, protocol behavior, API capability, configuration semantic, operational script, and bug fix must have appropriate automated regression coverage.
- A bug fix must begin with or include a test that fails before the fix and passes after it.
- New DNS behavior requires packet-level or integration coverage over both UDP and TCP where applicable.
- New REST functionality requires contract tests and shared-domain tests.
- New MCP functionality requires protocol tests and REST/MCP parity tests.
- New chaos behavior requires deterministic-seed tests, safety-cap tests, cancellation tests, and resource-leak tests.
- Configuration changes require valid, invalid, normalization, round-trip, and backward-compatibility tests.
- Never delete or weaken a test merely to make CI pass unless the test is provably incorrect; document the reason in the change.

## CI is mandatory

- All required CI checks must pass before merge and before a release tag is created.
- Do not bypass, skip, mark optional, or administratively override a failing check to ship a change.
- Treat every CI failure as either a product defect or a pipeline defect.
- When CI fails, fix the immediate cause and harden the system so that the same failure is easier to diagnose and less likely to recur.
- Hardening may include a regression test, better assertions, deterministic fixtures, explicit timeouts, improved diagnostics, dependency pinning, hermetic test setup, or narrowly justified retry logic for a proven transient external dependency.
- Do not hide flaky tests with broad retries. Find and remove the source of nondeterminism.
- A task is incomplete until all relevant local and CI-equivalent targets pass.

## Release tags and release notes

- Every release tag must include complete release notes describing all functionality differences from the previous release.
- Release notes must cover additions, behavior changes, bug fixes, removals, security changes, REST changes, MCP changes, DNS semantics, configuration/schema changes, chaos behavior, deployment changes, compatibility impact, migrations, and known limitations.
- A raw commit list or automatically generated pull-request list is not sufficient.
- Compare generated API schemas, MCP capability manifests, configuration schemas, defaults, and CLI help between tags to detect undocumented differences.
- Breaking changes require explicit migration guidance and the version increment required by the compatibility policy.
- Release notes and changelog entries are part of the release artifact and must be reviewed before tagging.

## Documentation is mandatory

- All documentation must be kept up to date.
- Update affected architecture, API, MCP, configuration, security, operation, testing, deployment, task, and ADR documents in the same change as the implementation.
- Stale documentation is a defect and blocks task completion.
- Examples must be tested or generated where practical.
- Internal links, code samples, configuration examples, and command lines must pass documentation checks.
- Update `Last reviewed` metadata when a document receives a substantive review.
- Do not change an architectural invariant without an ADR.

## REST and MCP parity

- Every public REST control capability must have an MCP equivalent.
- Every state-changing MCP tool must have a REST equivalent.
- Parameterized MCP read tools must have REST equivalents; MCP resources may mirror REST GET representations.
- Both adapters must use the same input and output domain types and the same authorization decision.
- Every mutation must support validation, dry-run planning, optimistic concurrency, idempotency, actor identity, reason, deterministic errors, audit emission, and an atomic commit.
- Run parity verification whenever REST, MCP, schemas, authorization, or application commands change.

## DNS correctness

- Wildcard behavior must follow `docs/02-dns-semantics.md`.
- Exact owner names take priority over wildcard synthesis.
- Empty non-terminals must be represented in the compiled name tree.
- Authoritative misses must not fall through to forwarding unless the zone is explicitly an overlay.
- UDP and TCP semantics must remain equivalent except for transport-specific behavior such as truncation or deliberate chaos.
- CNAME loops and invalid record combinations must be rejected or bounded as documented.
- Never set authoritative, recursion-available, authenticated-data, or truncation flags inaccurately.

## Chaos correctness and safety

- Chaos is disabled by default unless explicitly enabled in desired state and permitted by runtime policy.
- Every active chaos policy must have a stable ID, owner, reason, scope, safety class, activation state, and expiry unless a repository policy explicitly allows a permanent policy.
- Random decisions must be reproducible when a seed and deterministic mode are supplied.
- Delay, concurrency, drop probability, and resource use must obey global safety caps even if a policy requests more.
- Protected names, management addresses, and exempt client groups must never be affected.
- Emergency disable must be local, fast, authenticated where appropriate, and independent of the normal mutation path.
- `resolve:explain` and chaos simulation must report the matched policy and decision without changing live state.

## Generated files

- Do not manually edit generated OpenAPI, JSON Schema, MCP manifest, mocks, golden capability maps, or generated documentation.
- Change the source model or specification and run the documented generation target.
- Generation verification must leave the worktree clean.

## Dependencies

- Prefer the Go standard library and small, well-maintained libraries.
- Pin direct dependencies and review transitive changes.
- Hide DNS, MCP, telemetry, and schema-library types behind internal adapters.
- New dependencies require a justification in the pull request and must pass license and vulnerability checks.

## Required completion commands

The implementation repository should provide equivalent targets for:

```text
make format
make lint
make generate
make verify-generated
make test
make test-race
make test-fuzz-smoke
make test-integration
make test-parity
make test-config-compat
make test-docs
make test-container
make security-scan
make test-changelog
make web-test
make web-build
```

If a target does not yet exist, the task that first needs it must add it rather than silently omitting the check. Placeholders must fail closed, not succeed as no-ops.
