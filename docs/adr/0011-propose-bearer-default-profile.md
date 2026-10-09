# ADR 0011: Propose bearer as the default auth profile

Status: PROPOSED
Date: 2026-10-09

## Context

`config.Normalize` and `auth.NewPolicy` fill an empty `spec.management.auth.profile` with `dev-loopback-unauth` (`normalize.go:87-89`, `policy.go:42-44`). `Identify` then makes a loopback peer administrator (`identity.go:89-90`). The default listen is `:8080` on every interface (`defaults.go:11`). A same-host reverse proxy or SSH tunnel arrives as that loopback peer. ADR 0010 removed the loopback exception for `bearer` only. Matt Brewer, 2026-10-09: do not change the default in this change; propose `bearer` as the future default.

## Decision

1. This change does not change the default. Omitted profile remains `dev-loopback-unauth`. The startup warning and the docs/08 paragraph are the mitigation that ships now.
2. Proposed, not accepted: an empty profile normalizes to `bearer` in both `config.Normalize` and `auth.NewPolicy`. Explicit `dev-loopback-unauth` stays a valid profile and keeps today's loopback exception.
3. When that proposal is accepted, an empty profile with no `secretRef` fails closed the way `bearer` already does (`internal/config/validate.go:765-768`, `policy.go:60-63`).
4. Health live/ready stay unauthenticated. `X-Forwarded-For` stays untrusted. MCP stdio `LocalOrStdio` stays unchanged. The startup warning stays for an explicit `dev-loopback-unauth` bound beyond loopback.

## Consequences

Accepting the proposal changes who is administrator: loopback clients of a document that omitted the profile must send `Authorization: Bearer` (ADR 0010). Documents that omit the profile and have no token file fail at startup. Documents that already set `secretRef` but omit the profile today still treat loopback as administrator (`NewPolicy` loads the file for either profile, `policy.go:53-58`); after acceptance they become `bearer`. Canonical JSON materializes the profile, so those revisions change the same way omitted `spec.ui` did (`docs/16-compatibility-and-versioning.md` 1.1.0 note). No REST, MCP, or schema shape change. Semver at acceptance is the release owner's call; this proposal does not classify it. Expect a migration note, not a silent patch.

Until acceptance, operators only gain the stdout warning. Auth decisions are unchanged.

## Migration

Until acceptance, nobody moves. To keep today's behavior after acceptance, set `spec.management.auth.profile: dev-loopback-unauth` before upgrading. To take the new default, set `secretRef` to a token file (`openssl rand -hex 32`), restart, and send `Authorization: Bearer` from loopback, including the console.

`examples/labdns-deploy` `environments/main-lab/dns.yaml:44` and `environments/test-lab/dns.yaml:38` already set `profile: bearer` and a `secretRef`. They do not need an edit when this proposal is accepted. Both still set the process address to `:8080` (`main-lab/dns.yaml:14`, `test-lab/dns.yaml:14`). main-lab and `examples/compose.smoke.yaml` publish host `127.0.0.1:8080:8080`. test-lab publishes host `127.0.0.1:18080:8080/tcp`. That publish is not a loopback process bind. Under `bearer` the startup warning does not fire. This change does not edit those files. `testdata/container/config.yaml` and `examples/compose.smoke.yaml` omit the profile; they would need an explicit `dev-loopback-unauth` or a token file only after acceptance. Do not change them now.

## Alternatives considered

- Change the default in this PR: rejected for this change (Matt Brewer, 2026-10-09).
- Warn only for wildcard binds and stay silent for a specific non-loopback IP: rejected. One predicate, `auth.IsLoopback` on the bound address.
- Classify the YAML or flag string: rejected. `Listen` rewrites it.
- Trust `X-Forwarded-For`: rejected. The peer is the proxy's TCP connection.

## Review triggers

Acceptance of this proposal, or a later profile that understands a reverse-proxy identity.
