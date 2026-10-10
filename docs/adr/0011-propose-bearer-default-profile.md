# ADR 0011: Propose bearer as the default auth profile

Status: Accepted
Date: 2026-10-09

## Context

`config.Normalize` and `auth.NewPolicy` fill an empty `spec.management.auth.profile` (`normalize.go:87-89`, `policy.go:42-44`). `Identify` makes a loopback peer administrator only when the report is explicit `dev-loopback-unauth` (`identity.go:89-90`). The default listen is `:8080` on every interface (`defaults.go:11`). A same-host reverse proxy or SSH tunnel that dials the process from loopback in its network namespace arrives as that loopback peer (a Docker port publish does not: the peer is the bridge address). ADR 0010 removed the loopback exception for `bearer` only.

PR #60 (`muse/dns-loopback-exposure-warn`) left the omitted default as `dev-loopback-unauth` and shipped the startup warning. Acceptance is Matt Brewer, 2026-10-09 19:36 ET, relayed by Keystone: the next minor makes `bearer` the default.

## Decision

1. An empty profile normalizes to `bearer` in both `config.Normalize` and `auth.NewPolicy`.
2. Explicit `dev-loopback-unauth` stays a valid profile and keeps today's loopback exception. Health live/ready stay unauthenticated. `X-Forwarded-For` stays untrusted. MCP stdio `LocalOrStdio` stays unchanged. The startup warning stays for an explicit `dev-loopback-unauth` profile on a non-loopback bind.
3. The fail-closed path is `NewPolicy` / `bindManagementAuth`, not `Validate`. `Load` runs before `--management-listen=off` (`serve.go:115` then `:177`), so a schema requirement for `secretRef` would reject a management-off start. `labdns validate` reports only an empty `secretRef`. Unreadable and empty files fail at serve. One sentence, which is the domain error message: `bearer profile has no usable token; set spec.management.auth.secretRef to a token file, or set profile: dev-loopback-unauth`.

## Consequences

Loopback clients of a document that omitted the profile must send `Authorization: Bearer` (ADR 0010). Documents that omit the profile and have no token file fail at startup when management is bound. Documents that set `secretRef` and omit the profile become `bearer` (loopback is no longer administrator). Canonical JSON materializes `"profile":"bearer"`, so those revisions change the same way omitted `spec.ui` did (`docs/16-compatibility-and-versioning.md` 1.1.0 note). Documents that already set the profile keep the same revision. `hash-v1` does not hash the management profile.

No REST, MCP, or schema **shape** change. The config-schema description and the OpenAPI bearer description do change. Semver is minor v1.5.0 by this decision (2026-10-09): breaking for omitted-profile deployments, with a migration note. Not a silent patch and not a major.

## Migration

Operators who want today's behavior set `spec.management.auth.profile: dev-loopback-unauth` before upgrade. Operators who take the default set `secretRef` to a token file (`openssl rand -hex 32`), restart, and send `Authorization: Bearer` from loopback, including the console. Configs with `secretRef` and no profile become `bearer` (loopback admin goes away). `--management-listen=off` does not need a token. `labdns validate` reports an empty `secretRef` and does not open the file. Unreadable and empty token files fail at `labdns serve` with the same sentence.

The Compose smoke uses explicit `dev-loopback-unauth` (not a token file) so `scripts/test-container.sh` in-container docs checks keep working. main-lab and test-lab YAML stay as they are (`profile: bearer` and a `secretRef`). Both still set the process address to `:8080` (`main-lab/dns.yaml:14`, `test-lab/dns.yaml:14`). main-lab and `examples/compose.smoke.yaml` publish host `127.0.0.1:8080:8080`. test-lab publishes host `127.0.0.1:18080:8080/tcp`. That publish is not a loopback process bind. Under `bearer` the startup warning does not fire.

The #60 stack (`21b33a7..1f43e89`) edited comments and prose, not profiles or port mappings, in these seven files: `examples/compose.smoke.yaml`, `examples/labdns-deploy/environments/main-lab/compose.yaml`, `examples/labdns-deploy/README.md`, `examples/labdns-deploy/docs/operations.md`, `examples/labdns-deploy/docs/onboarding.md`, `examples/labdns-deploy/environments/main-lab/README.md`, and `examples/labdns-deploy/environments/test-lab/README.md`. The `dns.yaml` files were not edited.

This acceptance also edits `testdata/container/config.yaml` (explicit `profile: dev-loopback-unauth`), the comments in `examples/compose.smoke.yaml`, and one sentence in `examples/labdns-deploy/secrets/README.md`.

## Alternatives considered

- Do not change the default in the #60 PR: that was the decision then (Matt Brewer, 2026-10-09). This ADR accepts the follow-up. The next minor makes `bearer` the default.
- Warn only for wildcard binds and stay silent for a specific non-loopback IP: rejected. One predicate, `auth.IsLoopback` on the bound address.
- Classify the YAML or flag string: rejected. `Listen` rewrites it.
- Trust `X-Forwarded-For`: rejected. The peer is the proxy's TCP connection.

## Review triggers

A later profile that understands a reverse-proxy identity.
