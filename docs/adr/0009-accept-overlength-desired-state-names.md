# ADR 0009: Accept over-length DNS names in desired state

Status: Accepted
Date: 2026-08-23

## Context

RFC 1035 limits a DNS label to 63 octets and a wire-encoded name to 255 octets (presentation FQDN typically 254 characters plus the trailing root dot). LabDNS previously rejected desired-state names that exceeded those caps in `config.CanonicalName` / `checkDNSName`.

QA labs need to load owners, zone names, SOA/NS names, forwarding suffixes, chaos scopes, and name tokens in MX/SRV/SVCB/HTTPS RDATA that real resolvers or applications mishandle. Those names are useful as configured local data even though they cannot be packed into a syntactically valid DNS message.

ADR 0007 still forbids emitting malformed packets. Relaxing desired-state length checks must not teach the wire encoder to write illegal length prefixes.

## Decision

1. Desired-state name canonicalization accepts ASCII LDH+underscore names (and a leftmost `*` wildcard) without a 63-octet label cap or a 254-character presentation FQDN cap.
2. Non-length syntax stays fail-closed: empty name, empty label, non-ASCII, characters outside `a-z` `0-9` `-` `_` (and leftmost `*`), labels that start or end with `-`, and `*` that is not the leftmost label.
3. Over-length names are stored as lower-case FQDNs with a trailing dot and answer on the local/management resolve path (exact owner + type).
4. UDP/TCP encoding remains RFC 1035-correct. Incoming wire QNAMEs that exceed those maxima still fail to unpack. Pack failure for an over-limit owner is the ADR 0007 boundary, not a reason to forge bytes.

## Consequences

- REST, MCP, YAML, and compile all accept over-length names because they share `config.CanonicalName`.
- JSON Schema `$defs.name` already has no `maxLength`; the Go validator was the choke point.
- Operators can create QA names that cannot appear on the DNS wire. Management `resolve` / `explain` is the supported query path for those owners.
- Whole-document size remains capped at `MaxDocumentBytes` (1 MiB).

## Alternatives considered

- Restore the old length checks behind a config flag: rejected. Extra surface for a lab appliance; QA needs the names in ordinary desired state.
- Emit malformed wire for over-limit owners: rejected (ADR 0007).
- Accept non-ASCII / IDNA: out of scope.

## Review triggers

Review if an approved ADR later adds isolated malformed-wire generation, if IDNA is adopted, or if a product requirement appears to reject over-length names again.
