# RES-001: Local Resolver and Wildcards

Status: not-started
Recommended owner: DNS semantics agent
Dependencies: CFG-001, DNS-001
Exclusive ownership: `internal/compiler` zone indexes, `internal/resolver`

## Goal

Implement deterministic exact, wildcard, authoritative, overlay, CNAME, and negative-answer behavior over immutable compiled state.

## Work items

- [ ] Compile zone suffix trie and owner-existence tree including empty non-terminals.
- [ ] Compile owner/type RRset indexes and wildcard source indexes.
- [ ] Implement most-specific zone selection.
- [ ] Implement exact RRset resolution.
- [ ] Implement closest-encloser wildcard synthesis.
- [ ] Implement exact-over-wildcard and empty-non-terminal rules.
- [ ] Implement bounded CNAME processing and final-answer construction.
- [ ] Implement authoritative NXDOMAIN and NODATA with SOA.
- [ ] Implement overlay fallthrough signaling without forwarding inside the resolver package.
- [ ] Set authoritative and recursion-related response metadata for the wire layer.
- [ ] Produce structured resolution explanation including wildcard source and closest encloser.
- [ ] Preserve immutable RRsets when applying answer ordering.

## Required tests

- [ ] Table-driven exact record tests for all initial structured RR types.
- [ ] RFC wildcard examples and edge cases.
- [ ] Empty non-terminal regression matrix.
- [ ] Literal asterisk-label query tests.
- [ ] Wildcard CNAME tests.
- [ ] Rejection tests for wildcard DNAME and NS.
- [ ] Authoritative NXDOMAIN versus NODATA tests with correct SOA.
- [ ] Overlay fallthrough tests.
- [ ] CNAME loop/depth tests.
- [ ] UDP/TCP packet-level equivalence tests through DNS-001.
- [ ] Flag correctness tests.
- [ ] Snapshot immutability and race tests.
- [ ] Regression fixture for every semantic bug.

## Documentation updates

- [ ] Update `docs/02-dns-semantics.md` with implementation-confirmed behavior.
- [ ] Add explanation examples.
- [ ] Document supported RR types and known exclusions.
- [ ] Add release-note entry for local resolution semantics.

## Acceptance criteria

- All normative DNS semantic fixtures pass.
- Wildcard synthesis is explainable and standards-aligned.
- Authoritative misses never fall through.
- Overlay misses produce an explicit forwarding decision.

## Handoff

Provide the base resolution result structure consumed by forwarding, cache, chaos, REST, and MCP.
