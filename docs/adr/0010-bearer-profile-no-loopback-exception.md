# ADR 0010: Bearer profile has no loopback exception

Status: Accepted
Date: 2026-10-08
Amended: 2026-10-09 (a nil authenticator fails closed; an unreported profile loses only the loopback exception)

## Context

Q-AUTH (2026-08-15) allowed unauthenticated management only from `127.0.0.1` and `::1`, and required a bearer token from every other peer. The first-GA implementation applied that loopback exception to both auth profiles. Under `profile: bearer`, a loopback request with no `Authorization` header was still an administrator (`internal/auth.Identify`).

GitOps environments set `profile: bearer`. The process binds `:8080` and Compose publishes that port on the host loopback. A client that dials the process from loopback in the same network namespace (for example a same-host reverse proxy or tunnel) was administrator without the token; remote/bridge-network peers already needed a bearer.

## Decision

Matt Brewer, 2026-10-08: "require a token from local requests in dns bearer mode."

1. Under `profile: bearer`, a loopback peer (`127.0.0.1`, `::1`, and IPv4-mapped loopback) without a bearer is unauthenticated. The error and status match a remote peer (`401`, `authentication required`).
2. `dev-loopback-unauth` is unchanged: loopback without a bearer is administrator.
3. Health live and ready stay unauthenticated in both profiles.
4. MCP stdio `LocalOrStdio` is unchanged. It has no network peer.
5. A nil authenticator fails closed: every non-probe request is unauthenticated, including a presented bearer and including loopback. An authenticator that does not report a profile, or that reports an empty or unknown profile, is not the `dev-loopback-unauth` exception. Only an explicit `dev-loopback-unauth` report keeps loopback-without-a-bearer as administrator; a presented bearer is still checked by that authenticator. An omitted YAML profile still normalizes to `dev-loopback-unauth` (`config.Normalize`, `NewPolicy`), so configured deployments are unchanged. Production `labdns serve` passes `*auth.Policy` when management is bound, and does not construct REST or MCP when there is no canonical state to load a profile from. A wrapper in front of that policy must forward `Profile`. MCP stdio `LocalOrStdio` does not call `Identify`.

REST may still authenticate with a live `labdns_session` cookie created by a bearer (or by loopback under `dev-loopback-unauth`). MCP ignores cookies and needs `Authorization: Bearer`.

## Consequences

- Loopback scripts and `curl` against a `bearer` listener must send `Authorization: Bearer`.
- The console button "Continue as local administrator" returns 401 `authentication required` under `bearer`. The operator pastes a token. The button still works under `dev-loopback-unauth` when the TCP peer is loopback; through a Docker port publish the peer is the bridge address and it returns 401.
- `dev-loopback-unauth` and unauthenticated health probes are unchanged.
- A nil authenticator rejects every non-probe request, bearer or not. An authenticator that does not report a profile, or that reports an empty or unknown profile, loses only the loopback-without-a-bearer administrator exception; a presented bearer is still checked by that authenticator. An omitted YAML profile still normalizes to `dev-loopback-unauth` (`config.Normalize`, `NewPolicy`), so configured deployments are unchanged.

## Alternatives considered

- Plumb an explicit loopback flag into every `Identify` caller: rejected. A missed caller would fail open. The profile stays next to the token source.
- Hide the console button under `bearer`: deferred. That needs a profile discovery endpoint. The existing login error path already shows the 401 detail.

## Review triggers

Review if a later profile adds another loopback exception, or if MCP gains a session cookie.
