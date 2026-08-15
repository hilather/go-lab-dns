# Reviewer Checklist

## Design and scope

- [ ] Change matches a tracked task and normative design.
- [ ] Any invariant change has an ADR.
- [ ] Scope is coherent and no hidden public behavior is introduced.

## Architecture

- [ ] REST/MCP adapters contain no independent business logic.
- [ ] DNS uses one immutable snapshot per query.
- [ ] Candidate state is fully validated before atomic swap.
- [ ] Third-party protocol types remain behind adapters.
- [ ] Chaos cannot affect management or health paths.

## DNS correctness

- [ ] Exact, wildcard, authoritative, overlay, CNAME, negative, cache, and forwarding implications are reviewed.
- [ ] UDP and TCP behavior is covered where applicable.
- [ ] Flags, TTLs, RCODEs, EDE, and limits are correct.

## Chaos safety

- [ ] Scope, expiry, seed, action phases, and maximum impact are clear.
- [ ] Protected names/clients and global caps cannot be bypassed.
- [ ] Timers and transport effects are cancellable and bounded.
- [ ] Simulation is side-effect free.

## Security

- [ ] Authentication, authorization, input limits, and audit are correct.
- [ ] No secret or sensitive identifier leaks.
- [ ] Alternate answers and upstream changes obey allowlists.
- [ ] Management defaults remain private.

## Tests

- [ ] Every changed area has regression coverage.
- [ ] Bug fixes include a failing-before/passing-after test where practical.
- [ ] Negative, race, fuzz, leak, integration, parity, and compatibility tests are present as appropriate.
- [ ] No test was weakened to make CI pass.
- [ ] Any CI failure was fixed and hardened.

## Documentation and release

- [ ] All affected documentation is current.
- [ ] Examples and generated references pass checks.
- [ ] Compatibility and migration impact is documented.
- [ ] Unreleased notes describe externally visible behavior.
- [ ] A release tag would be able to list all functionality differences.

## Completion

- [ ] All required CI passes.
- [ ] Generated files are clean.
- [ ] Handoff is complete.
