# RES-001: Local Resolver and Wildcards

Status: done
Recommended owner: DNS semantics agent
Dependencies: CFG-001, DNS-001
Exclusive ownership: `internal/resolver/**` including `compile.go`, `internal/snapshot/zone_index.go`

## Goal

Implement deterministic exact, wildcard, authoritative, overlay, CNAME, and negative-answer behavior over immutable compiled state.

## Work items

- [x] Compile zone suffix trie and owner-existence tree including empty non-terminals.
- [x] Compile owner/type RRset indexes and wildcard source indexes.
- [x] Implement most-specific zone selection.
- [x] Implement exact RRset resolution.
- [x] Implement closest-encloser wildcard synthesis.
- [x] Implement exact-over-wildcard and empty-non-terminal rules.
- [x] Implement bounded CNAME processing and final-answer construction.
- [x] Implement authoritative NXDOMAIN and NODATA with SOA.
- [x] Implement overlay fallthrough signaling without forwarding inside the resolver package.
- [x] Set authoritative and recursion-related response metadata for the wire layer.
- [x] Produce structured resolution explanation including wildcard source and closest encloser.
- [x] Preserve immutable RRsets when applying answer ordering.

## Required tests

- [x] Table-driven exact record tests for all initial structured RR types.
- [x] RFC wildcard examples and edge cases.
- [x] Empty non-terminal regression matrix.
- [x] Literal asterisk-label query tests.
- [x] Wildcard CNAME tests.
- [x] Rejection tests for wildcard DNAME and NS.
- [x] Authoritative NXDOMAIN versus NODATA tests with correct SOA.
- [x] Overlay fallthrough tests.
- [x] CNAME loop/depth tests.
- [x] UDP/TCP packet-level equivalence tests through DNS-001.
- [x] Flag correctness tests.
- [x] Snapshot immutability and race tests.
- [x] Regression fixture for every semantic bug.

## Documentation updates

- [x] Update `docs/02-dns-semantics.md` with implementation-confirmed behavior.
- [x] Add explanation examples.
- [x] Document supported RR types and known exclusions.
- [x] Add release-note entry for local resolution semantics.

## Acceptance criteria

- All normative DNS semantic fixtures pass.
- Wildcard synthesis is explainable and standards-aligned.
- Authoritative misses never fall through.
- Overlay misses produce an explicit forwarding decision.

## Handoff

Provide the base resolution result structure consumed by forwarding, cache, chaos, REST, and MCP.
