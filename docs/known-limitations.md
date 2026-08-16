# Known limitations (first GA)

Honest residual for first GA, last reviewed against **1.0.0-rc.2**. These are not defects hidden from the notes; they are out of first-GA scope, operator steps that a given change does not perform, or documented bounds. See [docs/18-roadmap-and-non-goals.md](https://github.com/hilather/go-lab-dns/blob/main/docs/18-roadmap-and-non-goals.md) for deferred product work.

Last reviewed: 2026-08-16

## Tag and image publish

- `v1.0.0-rc.1` notes shipped before that tag existed. `v1.0.0-rc.2` is the first post-rc.1 tag and still does not publish `ghcr.io/hilather/labdns`.
- The GitOps template still uses an all-zero digest placeholder until that pin exists ([examples/labdns-deploy](https://github.com/hilather/go-lab-dns/blob/main/examples/labdns-deploy/README.md)).
- Application binaries built without ldflags report version `dev`. The notes version `1.0.0-rc.2` is the candidate identity for the tag/image, not the default `dev` string.
- Required GitHub Actions green-on-tag is enforced by `tag-gate`, not by this branch commit alone.

## Upgrade and rollback

- Public-surface diff for `v1.0.0-rc.2` is against `v1.0.0-rc.1`, not the empty tree.
- Upgrade from rc.1 is process restart onto this tag. Bootstrap YAML needs no migration. MCP `/mcp` on `serve` is the only operator-visible behavior delta.
- Rollback is process restart onto a previous image digest / desired-state pin (ADR 0003). Runtime mutations are discarded on restart.
- Tag-time SBOM (`go list -m -json all`) and provenance attestation are operator/release-pipeline outputs, not artifacts of this change.

## Security and audit

- Durable audit is an in-process ring plus an optional best-effort external hook. The hook is **not** fail-closed. Loss of the process loses the ring.
- `dev-loopback-unauth` is a development profile. Remote management requires bearer tokens; do not expose the management listener on a public path.
- Query names and client addresses are redacted by default. `spec.observability.logQNAME` is a debug gate.
- Vulnerability reporting is GitHub private advisories only ([SECURITY.md](https://github.com/hilather/go-lab-dns/blob/main/SECURITY.md)).

## Chaos

- Delay distributions are `fixed` and `uniform` only. Clipped-normal, log-normal, and exponential are follow-ons.
- ADR 0007: arbitrary malformed-wire generation is not in the production process.
- Kernel-level / general network impairment (`tc`/netem) is out of scope.
- Simulation never sleeps, writes cache, or consumes budgets; it is not a substitute for a packet-level experiment.
- Startup `--chaos-disable` / `LABDNS_CHAOS_DISABLE` cannot be cleared by YAML, reset, or emergency-enable.

## DNS and configuration

- Config API is `labdns.dev/v1alpha1` only. `internal/config.Migrator` is empty.
- Record types: A, AAAA, CNAME, TXT, MX, SRV, PTR, CAA, NS, SOA, SVCB, HTTPS, plus validated generic RDATA. Wildcard NS and wildcard DNAME are rejected.
- Overlay CNAME may terminate in a forwarded name, subject to the CNAME depth cap (default 8; zero means 8, not unlimited).
- Upstream transport is UDP and TCP. DoT / DoH / DoQ are not first GA.
- No host `/etc/resolv.conf` fallback. Empty `clientGroups` serves local zones and forwards to no one.
- IDs are user-supplied and immutable within an API version. Server-generated IDs are deferred.
- Canonical export materializes defaults and drops comments. A comment sidecar is deferred.
- Snapshot history is active + bootstrap + **one** previous generation.

## Control plane and operations

- MCP protocol **2026-07-28 only** (ADR 0006). Other protocol versions are rejected.
- MCP stdio is a developer adapter and is **not** in the production image.
- Unix-socket management is out of first GA. Emergency control #3 is `SIGUSR1`.
- Typed CRUD write routes are not added; plan/apply is the write path.
- Management resolve defaults to not consuming live chaos.
- Absolute QPS / p99 numbers are recorded by benches, not gated in CI (hardware varies).
- Default soak in CI is 2s. Long soak is opt-in (`-soak=30m` / `LABDNS_SOAK_DURATION`).
- The 24-hour (or longer) pre-GA soak in [docs/10-testing-strategy.md](https://github.com/hilather/go-lab-dns/blob/main/docs/10-testing-strategy.md) was **not** executed on this candidate. Only the 2s CI soak ran; 30m remains optional.
- glibc, systemd-resolved, and Windows resolver interop are manual lab checks against `testdata/interop/cases.json`. CI uses Go wire, optional `dig` when on `PATH`, and `net.Resolver` (`PreferGo`).
- `dig` interop is skipped when `dig` is not installed (`TestInteropFixturesDig`).

## Explicit non-goals (unchanged)

Full Internet recursion, public authoritative hosting, RFC 2136, AXFR/IXFR, DNSSEC signing, multi-replica runtime consensus, web UI, DHCP, client-facing DoH/DoQ, malformed-wire generation in-process, general network impairment, direct Git writes from the DNS process, and any internal database or hidden volume. See [docs/18-roadmap-and-non-goals.md](https://github.com/hilather/go-lab-dns/blob/main/docs/18-roadmap-and-non-goals.md).
