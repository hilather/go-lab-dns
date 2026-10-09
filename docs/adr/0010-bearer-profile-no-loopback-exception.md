# ADR 0010: Bearer profile has no loopback exception

Status: Accepted
Date: 2026-10-08
Amended: 2026-10-09 (a nil authenticator or missing auth profile fails closed)

## Context

Q-AUTH (2026-08-15) allowed unauthenticated management only from `127.0.0.1` and `::1`, and required a bearer token from every other peer. The first-GA implementation applied that loopback exception to both auth profiles. Under `profile: bearer`, a loopback request with no `Authorization` header was still an administrator (`internal/auth.Identify`).

GitOps environments set `profile: bearer` and bind management on loopback. That combination meant a process on the host, or anything that could open a TCP connection to the management listener, was administrator without the token file.

## Decision

Matt Brewer, 2026-10-08: "require a token from local requests in dns bearer mode."

1. Under `profile: bearer`, a loopback peer (`127.0.0.1`, `::1`, and IPv4-mapped loopback) without a bearer is unauthenticated. The error and status match a remote peer (`401`, `authentication required`).
2. `dev-loopback-unauth` is unchanged: loopback without a bearer is administrator.
3. Health live and ready stay unauthenticated in both profiles.
4. MCP stdio `LocalOrStdio` is unchanged. It has no network peer.
5. A nil authenticator fails closed: every non-probe request is unauthenticated, including a presented bearer and including loopback. An authenticator that does not report a profile, or that reports an empty or unknown profile, is not the `dev-loopback-unauth` exception. Only an explicit `dev-loopback-unauth` report keeps loopback-without-a-bearer as administrator. Production `labdns serve` passes `*auth.Policy` when management is bound, and does not construct REST or MCP when there is no canonical state to load a profile from. A wrapper in front of that policy must forward `Profile`. MCP stdio `LocalOrStdio` does not call `Identify`.

REST may still authenticate with a live `labdns_session` cookie created by a bearer (or by loopback under `dev-loopback-unauth`). MCP ignores cookies and needs `Authorization: Bearer`.

## Consequences

- Loopback scripts and `curl` against a `bearer` listener must send `Authorization: Bearer`.
- The console button "Continue as local administrator" returns 401 `authentication required` under `bearer`. The operator pastes a token. The button still works under `dev-loopback-unauth`.
- `dev-loopback-unauth` and unauthenticated health probes are unchanged.
- An unconfigured authenticator or a missing auth profile does not grant administrator, on loopback or remotely. Configured `dev-loopback-unauth` and `bearer` are unchanged.

## Alternatives considered

- Plumb an explicit loopback flag into every `Identify` caller: rejected. A missed caller would fail open. The profile stays next to the token source.
- Hide the console button under `bearer`: deferred. That needs a profile discovery endpoint. The existing login error path already shows the 401 detail.

## Review triggers

Review if a later profile adds another loopback exception, or if MCP gains a session cookie.
