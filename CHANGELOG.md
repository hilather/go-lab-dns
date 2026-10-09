# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Changed

- Go toolchain pinned to go1.26.9 (go.mod `toolchain`, CI and release `GO_VERSION`, Dockerfile); 1.26.0–1.26.8 lack current stdlib security fixes, most recently the Go 1.26.9 fixes (GO-2026-6603 to -6605, -6607 to -6613 and -6617 in `net/http`, `net/textproto`, `mime/multipart`, `crypto/tls` and `os`). `golang.org/x/net` moves from v0.58.0 to v0.60.0 (fixes the x/net side of GO-2026-6603, -6610, -6611, -6612 and -6617), which also pulls `golang.org/x/sys` v0.48.0; the go.mod language line is written as `go 1.26.0`.
- Web development dependency `source-map-js` updates from 1.2.1 to 1.2.2 (GHSA-68fv-2mgg-jv7q, high: event-loop denial of service through indexed source-map section offsets). Lockfile only; the `web/dist` build output is byte-identical, and the web CI `npm audit --audit-level=high` step passes again.

### Deprecated

- Management bearer tokens shorter than 32 bytes are deprecated. This release does not check plain-token length when `labdns serve` reads `spec.management.auth.secretRef` at process start (auth is serve-time; reset does not reread tokens). The next minor release refuses a token shorter than 32 bytes at that same load. Replacement: generate tokens with `openssl rand -hex 32` (the `examples/labdns-deploy` instructions now do). This release has no short-token log line or metric; the shared-auth (controlkit) migration adds a load-time warning (token id only) and the `labdns_auth_short_tokens_total` counter before enforcement.

### Security

- `profile: bearer` no longer treats loopback (`127.0.0.1`, `::1`) requests without a bearer as administrator; they get 401 like remote peers. Send `Authorization: Bearer` (or sign in to the console with a token). `dev-loopback-unauth` and unauthenticated health probes are unchanged. See [ADR 0010](docs/adr/0010-bearer-profile-no-loopback-exception.md).
- The management authenticator now fails closed when no authenticator or no auth profile is configured (P5). A nil authenticator, an authenticator that does not report a profile, and an empty or unknown profile are unauthenticated for every non-probe request. Configured `dev-loopback-unauth` and bearer profiles are unchanged, and health probes stay unauthenticated.

### Compatibility and migration

- Under `profile: bearer`, loopback clients (curl, scripts, the console button) must now send `Authorization: Bearer` (or use a REST session cookie created with a bearer). MCP must send the header. `dev-loopback-unauth` is unchanged. See [ADR 0010](docs/adr/0010-bearer-profile-no-loopback-exception.md).

## v1.4.1 — 2026-10-05

Curated notes: [docs/releases/v1.4.1.md](https://github.com/hilather/go-lab-dns/blob/v1.4.1/docs/releases/v1.4.1.md).

### Fixed

- Notes-only patch: no code, API surface, or behaviour change from v1.4.0. The v1.4.0 Release `tag-gate` failed because `docs/releases/v1.4.0.md` was missing at that tag. v1.4.1 ships those curated notes and this frozen CHANGELOG for v1.4.0.

## v1.4.0 — 2026-10-04

Curated notes: [docs/releases/v1.4.0.md](https://github.com/hilather/go-lab-dns/blob/v1.4.1/docs/releases/v1.4.0.md) (added after the v1.4.0 tag; first tagged in v1.4.1).

### Added

- Optional `baseAnswers` and `chaosDecisions` explanation fields in REST/MCP management resolve/explain results and the generated schemas (OpenAPI `ResolutionExplanation`, `ChaosDecision`); TTLs and decision delays remain duration strings.

### Changed

- Configuration validation now enforces positive `maxActiveHighImpactPolicies` against enabled high-impact policies and rejects negative delayed-concurrency, high-impact-count, and default-lifetime safety limits and negative policy budget limits. Positive `budget.maxFrequency` is rejected because the runtime does not implement that cap; omit it or set it to zero.
- Enforce aggregate drop probability across composed policies and both execution groups by allocating an immutable snapshot-wide selector budget. Pressure drops and inactive policies reserve shares too; mixed outcomes, disjoint policies, and policies dropping in both phases may trigger less often under a positive cap. Canonical export, hash-v1 encoding, and random draw counts are unchanged; action-plan clamp evidence reports effective thresholds. Zero remains unlimited and cap one leaves selectors unchanged.
- Deployment chaos verification now rejects unlimited zero delay/concurrency caps when delays are configured, checks actual drop-policy probabilities, and counts enabled high-impact policies directly against deployment ceilings.
- Reject unsupported REST paths and MCP paths that collide with REST/operator-console routes or contain mux patterns. REST remains `/v1`; distinct MCP prefixes remain supported.
- Changelog enforcement covers operator web application changes and Go/frontend dependency manifests and locks while excluding test-only files and build output.
- Web development dependency `undici` updates from 7.29.0 to 7.29.1; required web CI now audits the complete dependency lockfile for high and critical vulnerabilities.

### Fixed

- Correct numerical edge handling without changing hash-v1 inputs or draws: probability one always triggers, uniform delays remain inside their half-open range even at maximum 64-bit draws, and finite weighted outcomes remain correctly proportional when their sum overflows. Interior finite mappings are unchanged.
- Chaos delay and pressure bookkeeping now discards released or expired idle policy entries. Unlimited-rate pressure policies no longer retain unnecessary request timestamps.
- Configuration JSON now rejects trailing closing delimiters, label keys named after duration fields round-trip unchanged, and duration formatting handles the minimum signed 64-bit duration without recursive overflow.
- Configuration schema inspection now works in standalone binaries and the scratch container; the published schema is embedded directly in the executable.
- Deployment verification now checks actual alternate IPv4/IPv6 action values against `AllowedAlternateAddresses`, including disabled policies. Emptying the desired-state address allowlist no longer bypasses deployment address policy.
- Composed chaos delays now obey cumulative global and per-policy delay caps and every contributing policy's concurrency cap. Reservations still use one global slot per delayed query and release all policy slots on completion or cancellation.
- Lower-precedence chaos outcomes can no longer replace a selected conflicting transport action within or across resolution phases. Within one decision phase, live evaluation and simulation report skipped conflicting outcomes as `transport_conflict`.
- DNS admission preserves reserved QCLASS zero and returns NOTIMP over UDP and TCP instead of treating it as IN.
- Forwarding rejects upstream packets whose QR, opcode, question count, name, type, or class does not match the request. Invalid and incomplete truncated replies follow transport-error failover and otherwise return SERVFAIL; disabling TCP retry no longer turns a partial reply into a complete cacheable answer.
- Canceling an in-flight upstream exchange now closes its socket immediately instead of waiting for the attempt deadline.
- Negative caching requires an SOA and respects SOA MINIMUM (including zero) and detects CNAME chains ending in NXDOMAIN or SOA-backed NODATA, including management resolve cache writes.
- REST mutations with optional bodies reject trailing JSON values and delimiters before invoking reset, cache flush, or emergency controls. REST/MCP duration formatting handles the minimum signed duration without recursion overflow.
- Successful bootstrap reset cancels outstanding chaos delay reservations; failed reset leaves existing delays intact. Cache flush now enforces administrator authorization and records successful and denied calls in the shared audit ring.
- Operation decoding preserves arbitrary chaos policy label values named `ttl`, `duration`, or other duration field names.
- REST/MCP plans and mutations accept documented duration strings for DNS editor records and chaos designer policies; authorization no longer mistakes valid TTL/delay strings for protected-object or invalid-policy errors.
- DNS semantics and chaos safety documentation are embedded in the binary so REST/MCP documentation endpoints and the operator console work in production containers without a source checkout. Docker build context includes only the documentation Go package and its two canonical embed inputs; automated packaging and in-container endpoint checks guard against missing embed sources.
- Atomic plan/apply/validate batches now authorize each operation against preceding candidate changes. This prevents low-privilege actors from activating a newly added high-impact chaos policy or adding a relative protected owner through a newly added zone. REST and MCP share the fix and report complete required permissions.
- Kubernetes rollback restores the snapshotted Kustomize image digest alongside bootstrap YAML and image.env, so image-pin rollback validates and deploys the previous image.
- Kubernetes deployment scripts explicitly restart pods after applying manifests so unchanged desired state reloads bootstrap and discards runtime drift; failed deployments preserve both successful snapshots, so rollback still restores the previous successful deployment after apply, restart, or rollout failure.
- Release CI verification rejects missing commit SHAs and pending reruns instead of reusing an older successful check. GitHub check-run IDs identify the current attempt.
- Architecture documentation now describes the implemented operator console and the current DNS wire adapter dependency.
- Wire shared upstream health and DNS/REST/MCP/application metrics in production; synthetic chaos faults no longer poison upstream health. Stop runtime signal workers on shutdown and failed startup.
- Apply cache policies immediately across DNS and management queries, clear disabled caches, and reject stale-generation cache writes. Reset restores bootstrap cache policy.
- Management resolve with `applyChaos: true` and explain now model active chaos without executing delays, forwarding, or live chaos mutations. Resolve may cache the base result, while explain bypasses the cache. Cached explanations use the current caller and transport.
- Keep the displaced snapshot pointer monotonic under concurrent emergency and normal publications.
- Snapshot publication keeps generation monotonic when emergency controls advance it during compilation; apply/reset results report the actual published generation.
- Forwarding runtime health and round-robin counters discard removed topology IDs instead of retaining every historical deployment. Late old-snapshot exchanges cannot restore retired state; retargeting an upstream endpoint or transport clears its previous health. Caller cancellation and simulated upstream failures do not mark real upstreams unhealthy; expiring the total query deadline while waiting on an upstream still counts against it.
- Operator console cancels and clears cached session, audit, and state data on identity transitions; late session responses cannot replace a newer CSRF secret or restore an old actor after logout.
- Serialize browser session cookie mutations and revoke superseded logins before recovery; failed logout cleanup stays fail closed while explicit bearer sign-in can recover.
- Shell revision uses the shared status query and refreshes after mutations, promptly discarding stale plans. Changes permits forwarder and chaos mutation scopes and built-in editor/operator roles while REST still authorizes each operation.
- Chaos activation retries reuse idempotency keys only for the same request; edits to reason, expiry, policy, or revision get a new key.
- Forwarder comments and DNS semantics documentation now state that an expired query deadline during an upstream exchange answers SERVFAIL; only cancellation (shutdown, TCP peer close) or a deadline that expired before handling sends no answer.

## v1.3.2 — 2026-09-21

Curated notes: [docs/releases/v1.3.2.md](https://github.com/hilather/go-lab-dns/blob/v1.3.2/docs/releases/v1.3.2.md).

### Changed

- Direct dependency `github.com/modelcontextprotocol/go-sdk` 1.7.0 → 1.8.0. MCP cancel test polls for async `notifications/cancelled` delivery under go-sdk 1.8.

## v1.3.1 — 2026-09-07

Curated notes: [docs/releases/v1.3.1.md](https://github.com/hilather/go-lab-dns/blob/v1.3.1/docs/releases/v1.3.1.md).

### Fixed

- Refuse-forward (unknown / local-only) overlay CNAME answers are no longer stored under the shared local cache key. An unknown query first used to leave a CNAME-only hit that later `AllowForward` clients reused, skipping upstream Exchange until TTL expiry. Unknown and local-only clients still receive the local CNAME (RA=0). Fixes #42.
- Overlay CNAME fallthrough now forwards using the CNAME target's suffix policy when the original QNAME matches no forwarding suffix (typical suffix-only configs with no root `.` policy). Clients receive CNAME plus the upstream answer instead of a CNAME-only NOERROR. Unknown and local-only clients are unchanged. Fixes #41.
- Management `resolve` / `dns_resolve` with `useCache` no longer stores overlay `Fallthrough` results in the shared process cache. Those incomplete answers previously occupied the local cache key, so live DNS skipped forwarding and returned CNAME-only or empty NOERROR until TTL expiry. Fixes #40.
- DNS editors can no longer delete or retarget a protected name by replacing the whole zone (omitting the record) or by using a relative owner that expands to a protected FQDN. Plan/apply/validate now expand owners against the zone origin and deny zone update/remove when the current zone already contains a protected record. Fixes #38.
- DNS editors can no longer add a wildcard whose parent is an ancestor of a protected name (`*` in the same zone, or `*.lab.example.net.` when `dns.lab.example.net.` is protected). Closest-encloser synthesis would otherwise answer that QNAME without an exact owner. Fixes #39.

## v1.3.0 — 2026-08-29

Curated notes: [docs/releases/v1.3.0.md](https://github.com/hilather/go-lab-dns/blob/v1.3.0/docs/releases/v1.3.0.md).

### Changed

- Operator console shell is LabDNS dark. Primary nav is grouped Inspect / Mutate / Ref and includes Reset. The header shows one emergency verb (Disable when chaos is live, Enable when inhibited).
- `/login` and the remaining operator pages (`/`, `/state`, `/changes`, `/zones/:zoneId/records/:recordId`, `/resolve`, `/forwarding`, `/cache`, `/chaos`, `/chaos/:policyId`, `/audit`, `/audit/:eventId`, `/schema`, `/docs`, `/docs/:id`, `/capabilities`, `/reset`) use the same charcoal/amber chrome as `/zones` (`#0d0d0c` / `#161614` / `#f2efe6`, accent `#e09a3e`). Mutations still enqueue via `/changes`.
- `/zones` is an inventory rail plus selected-zone records (Owner, Type, TTL, Values, Chaos, ID). Create/Edit/Delete and Create record hop to `/changes` with operations prefilled. The `mutations in UI-003` stub is removed.

### Fixed

- Unknown `/zones/:zoneId` shows one `not_found` alert (not two from zoneQuery + recordsQuery). Fixes #34.
- Operator image `npm run build` (`tsc --noEmit`) no longer typechecks Vitest files, so `chrome.test.ts` `node:fs` imports do not fail the container web build.
- Operator README MCP (`/mcp`) section now matches ADR 0006 and [docs/07-mcp-api.md](docs/07-mcp-api.md): default pin remains **2026-07-28** (`Mcp-Protocol-Version` required); `spec.management.mcp.allowLegacyClients: true` skips the HTTP pin so MCPJungle can initialize without that header.

## v1.2.0 — 2026-08-23

Curated notes: [docs/releases/v1.2.0.md](https://github.com/hilather/go-lab-dns/blob/v1.2.0/docs/releases/v1.2.0.md).

### Changed

- Desired-state DNS names are no longer rejected solely because a label is longer than 63 octets or the presentation FQDN is longer than 254 characters. Over-length names (zone name, record owner, SOA/NS names, forwarding suffix, chaos protected/scope owners, and name tokens in MX/SRV/SVCB/HTTPS RDATA) load as lower-case FQDNs with a trailing dot and answer on the local/management resolve path. RFC 1035 wire maxima still apply to UDP/TCP messages: LabDNS does not emit malformed packets (ADR 0007). Non-length name syntax is unchanged (empty labels, non-ASCII, invalid characters, leading/trailing `-`, misplaced `*`). See [ADR 0009](docs/adr/0009-accept-overlength-desired-state-names.md).

## v1.1.1 — 2026-08-23

Curated notes: [docs/releases/v1.1.1.md](https://github.com/hilather/go-lab-dns/blob/v1.1.1/docs/releases/v1.1.1.md).

### Added

- Additive `spec.management.mcp.allowLegacyClients` (default **false**). When true, skip the HTTP-level MCP `2026-07-28` pin so older SDK clients (MCPJungle) can initialize. Does not add a claimed protocol version. Default remains 2026-07-28 only (ADR 0006).

### Changed

- Direct dependency `github.com/miekg/dns` 1.1.72 → 1.1.73.

## v1.1.0 — 2026-08-20

Curated notes: [docs/releases/v1.1.0.md](https://github.com/hilather/go-lab-dns/blob/v1.1.0/docs/releases/v1.1.0.md). rc.1/rc.2 notes were not rewritten.

### Added

- Embedded operator console on the management listener (`GET /` on `:8080`). Every `PARITY_REQUIRED` capability is completable: login/session, dashboard, state/export, schema, capabilities, docs, zones/records, resolve/explain, forwarding/cache, chaos (simulate/activate/deactivate/expiry/emergency), audit, changes (validate → plan → apply), reset, and cache flush. The UI is a REST client only (ADR 0008).
- Additive v1alpha1 `spec.ui.enabled` (default **true** when omitted, including documents with no `spec.access`; explicit `false` is preserved) and `spec.management.allowedOrigins` (exact `http(s)://host[:port]` Origin strings). New update-only target kind `ui` (`dns.admin`) so plan/apply/export cover UI drift.
- Capability registry `ui` bindings (SPA `route` and `view`/`mutate` `action`) on existing REST/MCP rows plus REST-only `session` and `ui.assets` (`TableRowCount` 33). Disposition is derived from `RESTOnly`. `POST`/`GET`/`DELETE /v1/session` issue HttpOnly cookie `labdns_session` and CSRF `X-LabDNS-CSRF`. Bearer wins over cookie. Cookie-present `POST /v1/session` without Bearer rotates the existing Actor and does not call Identify.
- Operator console login, shell (nav, revision, ready/degraded, emergency control), and status dashboard. `cmd/labdns serve` injects `web.Handler()` so non-loopback `GET /` returns HTML without a bearer while `GET /v1/state` remains 401. CSRF stays in module memory; bearer tokens are discarded after `POST /v1/session`.
- Generated OpenAPI TypeScript client (`make web-generate` writes committed `web/src/api/openapi.d.ts`; `make generate` stays Go-only). `openapi-fetch` sends `credentials: include` and CSRF on non-GET. TanStack Query keys include snapshot revision; status polls every 2s while visible; upstream/cache/chaos status poll every 5s; snapshot queries invalidate on revision change.
- Operator console chaos mutations: side-effect-free simulate, typed activate/deactivate/expiry, and shell emergency disable/enable. High-impact activate uses the same `CanActivateHigh` split as REST (`dns.chaos.activate` and `dns.chaos.emergency` or `dns.admin`); REST remains the authorizer. Emergency is one confirm, not a typed phrase.
- Structured log event `ui.session` (`EventUISession`) in the metrics catalog, including log field `actor_id` (token or identity id; not a metric label). Session routes use `labdns_capability_calls_total{capability="session"}`. No SPA request metrics.
- Vite/React operator-console toolchain (`web/`), committed embed stub (`internal/web`), fail-closed `make web-test` / `make web-build` / `make web-e2e`, required CI job `web`, and a digest-pinned Node 22.14.0 image stage. Production assets are copied into the embed path in Docker only; local `go test` uses the stub and does not require Node.
- Playwright operator-matrix e2e (`web/e2e`, `make web-e2e` / `npm run test:e2e`) against loopback `labdns serve` and `testdata/web/` (pack-sample plus viewer vs admin tokens). CI job `web` installs Chromium (`npx playwright install --with-deps chromium`) and runs the matrix. `make web-test` stays Vitest-only.
- Operator onboarding: open `http://127.0.0.1:8080/`, loopback Continue as local administrator or paste a bearer token, `spec.ui.enabled`, and `spec.management.allowedOrigins` in [docs/11-deployment.md](https://github.com/hilather/go-lab-dns/blob/v1.1.0/docs/11-deployment.md), [docs/12-deployment-repository.md](https://github.com/hilather/go-lab-dns/blob/v1.1.0/docs/12-deployment-repository.md), [docs/13-operations-and-runbooks.md](https://github.com/hilather/go-lab-dns/blob/v1.1.0/docs/13-operations-and-runbooks.md), and [examples/labdns-deploy](https://github.com/hilather/go-lab-dns/blob/v1.1.0/examples/labdns-deploy/README.md). GitOps Compose maps `127.0.0.1:8080:8080`. Hashed `web/dist` is not a `release-diff` surface.

### Changed

- Canonical JSON materializes `spec.ui.enabled: true` when `spec.ui` is omitted. Content-addressed `sha256:` revisions therefore change for previously hashed documents that omitted `spec.ui`. Operators with stored `expectedRevision` from 1.0.0-rc.* must re-GET state after upgrade. Chaos `hash-v1` vectors are unaffected.
- Operator docs: plan/apply of `spec.ui.enabled` and `spec.management.allowedOrigins` is live on the next request; `spec.management.auth` stays serve-time and needs a restart (in-process sessions drop).

### Fixed

- REST/MCP `EmergencyDisableChaos` now cancels outstanding chaos delays, matching `SIGUSR1` / `chaos.EmergencyDisable` and the documented emergency-disable contract.
- Plan idempotency cache hits re-check `expectedRevision` against the live snapshot and recompute after a foreign mutation, so a stale plan cannot be replayed. Empty `expectedRevision` is still rejected on cached Plan and Apply.
- Overlay CNAME answers are no longer discarded when forwarding is refused (unknown/local-only client, or CNAME target with no matching policy). Clients receive the local CNAME with NOERROR and RA=0.
- Wildcard synthesis after an exact CNAME now reports `source=wildcard` plus wildcard-source and closest-encloser explain fields.
- Chaos `exclusive-group` winners are shared across pre-resolution and response `Decide` calls on the same query.

### Removed or deprecated

- None.

## v1.0.0-rc.2 — 2026-08-16

Curated notes: [docs/releases/v1.0.0-rc.2.md](https://github.com/hilather/go-lab-dns/blob/v1.0.0-rc.2/docs/releases/v1.0.0-rc.2.md).

### Added

- `serve` mounts the MCP Streamable HTTP adapter on the management listener at `spec.listeners.management.mcpPath` (default `/mcp`), sharing the address and bearer policy with REST. New `rest.Config.Mounts` serves additional handlers on the management listener under the same lifecycle.

### Changed

- Add a cinematic README header image (`docs/assets/header.jpg`) and a 1280×640 social card (`docs/assets/social.jpg`).
- Rewrite the root README as an operator-facing product page: YAML bootstrap, CLI validate/serve, REST and MCP state-loading APIs, and a complete documentation map that links every architecture doc, ADR, and task list. Remove leftover agent-pack wording.
- Go 1.27 was evaluated on 2026-08-16 and deferred: only release candidates (up to `go1.27rc3`) exist. The pin stays **1.26.6** (already on the `v1.0.0-rc.1` tag SHA).
- Bump indirect modules: `golang.org/x/net` v0.55.0 → v0.58.0, `x/sys` v0.45.0 → v0.47.0, `x/sync` v0.20.0 → v0.22.0, `x/mod` v0.33.0 → v0.40.0, `x/tools` v0.42.0 → v0.49.0, `x/oauth2` v0.35.0 → v0.36.0. Direct dependencies were already current.
- CI workflows pin actions by commit SHA (`actions/checkout` v7.0.1, `actions/setup-go` v7.0.0, `actions/upload-artifact` v7.0.1, replacing Dependabot PRs #21–#23 floating v7 tags and the previous v4/v5 pins) and read the toolchain from one `GO_VERSION` workflow env var instead of scattered per-job pins.
- `golangci-lint` again runs the full v2 `standard` preset (errcheck, govet, ineffassign, staticcheck, unused) after the post-rc.1 govet-only stopgap; the stack leftovers it flagged are fixed: deferred `Close` errors now explicitly discarded, deprecated `parser.ParseDir` replaced by test-only `internal/testutil/goparse.ParseDir`, dead helpers removed (`bearerToken` wrappers, unused MCP test scaffolding, duplicate `isTransportAction`), and De Morgan / tagged-switch / copy-loop cleanups applied.

### Fixed

- Lint-driven test hardening: the cache test now asserts `Original` stays empty after `Get` (was an empty branch), the rate-limit burst test consumes its two tokens in separate statements so the assertion is explicit, and the classify-panic inflight test waits a full second for the follow-up SERVFAIL so a busy `go test ./...` run is not reported as a leak.

### Removed or deprecated

- None.

## v1.0.0-rc.1 — 2026-08-16

### Added

- GA-001 1.0.0-rc.1: acceptance evidence index, curated release notes, known limitations, and security-reporting details. Program board FND-001–GA-001 marked done. Image publish still waits for `ghcr.io/hilather/labdns` digest pin after this tag.
- Initial architecture and implementation plan.
- Repository foundation: Apache-2.0 license, Go 1.26 module `github.com/hilather/go-lab-dns`, package tree, Makefile, and fail-closed CI skeleton.
- Canonical domain types in `internal/model`: `State`/`Spec`, zones, records, forwarding, chaos policies, `Operation`/`Target`, and `Query`/`Result`.
- Stable domain error catalog in `internal/domainerr` matching [docs/17-error-model.md](https://github.com/hilather/go-lab-dns/blob/main/docs/17-error-model.md).
- Immutable `snapshot.Snapshot` shells and a working atomic `snapshot.Store` (`Load`/`Swap`/`Bootstrap`/`Previous`).
- DNS UDP/TCP listeners in `internal/dnsserver` with admission limits, TCP framing/deadlines, graceful shutdown, and chaos transport hints (`send`/`drop`/`truncate`/`tcp-close`/`tcp-reset`/`hold-then-close`).
- `github.com/miekg/dns v1.1.72` pinned behind `internal/dnswire`; library types do not escape the adapter.
- Compiled `snapshot.ZoneIndex` (existence tree, RRsets, wildcards, longest-suffix `Select`) filled by `resolver.Compile`. `Resolve` consumes a pre-selected zone ID.
- Compiled `snapshot.ForwardingIndex` (longest-suffix policies, default `.`) filled by `forwarder.Compile`. `Exchange` consumes a pre-selected policy ID.
- Process-scoped positive/negative `internal/cache` namespaced by snapshot revision, with TTL clamps, LRU eviction, and chaos lookup hooks.
- `internal/dnsquery` orchestrator (`dnsserver.Handler`): classify → pre `chaos.Decide` → resolve → optional exchange → post `chaos.Decide`. CHA-002 executes delay, RCODE/TTL/answer/EDE, transport hints, and cache/upstream/pressure hooks.
- Compiled `snapshot.AccessIndex` (longest-prefix CIDR → client group) filled by `snapshot.CompileAccess`.
- `compiler.Compile` orchestrates normalize/validate, zone/forwarding/chaos/access indexes, and `sha256:` revision hashing.
- `labdns serve --config PATH` loads bootstrap YAML, compiles an immutable snapshot, and binds UDP/TCP DNS. Invalid bootstrap does not listen.
- `internal/app.Service` mutation core: `Plan`/`Apply`/`Validate`/`Export`/`Reset`, zone/record/resolve/explain queries, forwarding/cache views, in-memory audit ring, and `EmergencyDisableChaos`.
- Frozen capability registry in `internal/capabilities` covering every first-GA REST↔MCP table row. Health live/ready are REST-only (not tools). Generated manifest: [api/capabilities/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/capabilities/v1.json).
- Domain-error mapping helpers: `domainerr` → RFC 9457 `application/problem+json` status hints and MCP JSON-RPC `data`. Parity harness fails if a table row is missing or renamed.
- REST `/v1` adapter in `internal/control/rest`: routes registered from the capability registry, `application/problem+json`, loopback-only unauthenticated management (Q-AUTH), body/timeout/concurrency limits, and generated OpenAPI at [api/openapi/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/openapi/v1.json).
- MCP Streamable HTTP adapter in `internal/control/mcp`: official Go SDK [`github.com/modelcontextprotocol/go-sdk v1.7.0`](https://github.com/modelcontextprotocol/go-sdk), protocol **2026-07-28 only**, tools and resources from the shared registry, four pack-07 prompts, structured JSON-RPC errors via `capabilities.JSONRPCFrom`, and generated manifest at [api/mcp/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/mcp/v1.json). Health live/ready are not tools. Stdio is a developer adapter only.
- REST `/v1` adapter in `internal/control/rest`: routes registered from the capability registry, `application/problem+json`, loopback-only unauthenticated management (Q-AUTH), body/timeout/concurrency limits, and generated OpenAPI at [api/openapi/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/openapi/v1.json). MCP is not implemented.
- Versioned metrics/events catalog (`labdns.dev/metrics/v1alpha1`) with a label allowlist, automated no-QNAME/no-client-IP checks, bounded export queues, structured JSON logs, optional sampled tracing, and a filled `app.Status` DTO (revisions, listeners, cache, upstreams, chaos, ready/degraded, warnings). Artifact: [api/metrics/v1alpha1.json](https://github.com/hilather/go-lab-dns/blob/main/api/metrics/v1alpha1.json).
- Production CLI: `serve`, `validate`, `canonicalize`, `verify`, `query`, `healthcheck`, `version`, and `chaos emergency-disable --pid-file`.
- Hardened multi-stage image `ghcr.io/hilather/labdns` (scratch, UID 65532, Apache-2.0 OCI labels, no shell).
- Shared SEC-001 identity, RBAC, abuse limits, and audit in `internal/auth` and `internal/audit`. Role×capability matrix, resource-aware plan/apply, separate chaos design/activate/high-impact/emergency privileges, management and DNS query rate limits, Origin/CORS deny-all, secret redaction, and an in-memory audit ring with a best-effort external hook (no fail-closed durable sink).
- Versioned metrics/events catalog (`labdns.dev/metrics/v1alpha1`) with a label allowlist, automated no-QNAME/no-client-IP checks, bounded export queues, structured JSON logs, optional sampled tracing, and a filled `app.Status` DTO (revisions, listeners, cache, upstreams, chaos, ready/degraded, warnings). Artifact: [api/metrics/v1alpha1.json](https://github.com/hilather/go-lab-dns/blob/main/api/metrics/v1alpha1.json).
- REL-001 release automation: `scripts/release-diff` compares OpenAPI, MCP manifest, config schema, capability table, metrics catalog, CLI help, error catalog, and chaos-effect catalog between two git refs. Tag notes live at `docs/releases/<tag>.md`. Generated CLI help: [api/cli/help.txt](https://github.com/hilather/go-lab-dns/blob/main/api/cli/help.txt). Generated error catalog: [api/errors/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/errors/v1.json).
- PERF-001 baselines: Go benches in `benches` (exact / wildcard / negative / cache hit / cache miss / upstream / chaos triggered / chaos idle), CI-safe soak (`-soak`, default 2s), max delayed concurrency, flood/admission, and client interop fixtures under [`testdata/interop`](https://github.com/hilather/go-lab-dns/blob/main/testdata/interop).
- Copyable GitOps template at [examples/labdns-deploy](https://github.com/hilather/go-lab-dns/blob/main/examples/labdns-deploy/README.md): Compose + Kubernetes, digest-pinned `ghcr.io/hilather/labdns`, isolated management, bearer `secretRef`, policy allowlists, and a probe suite (exact, wildcard, authoritative miss, overlay, unknown-client RA=0 / REFUSED, chaos simulation).
- `labdns verify` runs probes through the DNS orchestrator and accepts `--policies`, `--image` / `--image-env`, and `--server` for live probes. Probe schema: [api/jsonschema/labdns.dev.probes.v1alpha1.json](https://github.com/hilather/go-lab-dns/blob/main/api/jsonschema/labdns.dev.probes.v1alpha1.json).

### Changed

- None.

### Fixed

- None.

### Removed or deprecated

- None.

### Security

- Data-plane refuse-forward: unmatched clients are not an open recursive resolver. Local answers still serve.
- Management: loopback unauthenticated (`dev-loopback-unauth`); remote bearer required. Shared REST/MCP authorization. Protected objects and safety caps require administrator. Secrets are redacted from export, diffs, and audit payloads.
- DNS per-source query rate limit (default 256/s, burst 512) on top of existing inflight/TCP caps. Management default 32/s burst 64 plus 256 concurrent requests.

### DNS behavior

- UDP and TCP listeners admit a single IN class QUERY, echo ID/question, set QR, and apply FORMERR/NOTIMP/BADVERS as documented in [docs/02-dns-semantics.md](https://github.com/hilather/go-lab-dns/blob/main/docs/02-dns-semantics.md).
- Local resolver (`internal/resolver`): exact RRsets, RFC 4592 closest-encloser wildcards, empty non-terminals, bounded CNAME (default depth 8), authoritative NXDOMAIN/NODATA+SOA, overlay fallthrough. Overlay CNAME may terminate in a forwarded name (`Fallthrough=true`); the resolver does not forward. AA only on authoritative local/negative answers; AD never set; CD cleared locally. Zero `CNAMEDepth` falls back to 8, not unlimited.
- Suffix forwarding (`internal/forwarder`): ordered / round-robin / random / health-aware pools; UDP exchange with optional TCP retry after TC; failover only when `FailoverSpec` bools are set. NXDOMAIN does not fail over. Zero `timeout` is a 500ms per-attempt budget (250ms connect), stacked under the 2s query timeout, not unlimited. No `/etc/resolv.conf` fallback. Forwarded answers: AA=0, AD=0, CD pass-through.
- Refuse-forward: unknown or `AllowForward=false` clients get local answers with RA=0 and never hit upstreams. REFUSED only when there is no local path. Empty `clientGroups` serves local zones and forwards nothing. Pack-sample `ns1.lab.example.net` from 127.0.0.1 succeeds; a forward-only name from an unmatched IP is REFUSED with zero upstream packets.

### Chaos behavior

- Compiled `snapshot.ChaosIndex` (record/wildcard/owner/zone/forwarding/pool/group/global) filled by `chaos.Compile`.
- Deterministic selector `hash-v1` (SHA-256, ten length-prefixed fields, `u0`/`u1` uniforms). Goldens in [testdata/hash-v1/vectors.json](https://github.com/hilather/go-lab-dns/blob/main/testdata/hash-v1/vectors.json) include `timeBucket: 1s` same UTC second vs next second.
- `Engine.Decide` / `Simulate` use pre-classified client-group, zone, and forwarding IDs. Simulation never sleeps, writes cache, or consumes budgets.
- Gates: enabled, start/expiry, periodic flap, every-Nth. Composition: compose / terminal / exclusive-group.
- Global and per-policy delay/concurrency budgets with reservation/release. Protected names, protected client groups, and `chaosExempt` skip execution.
- `ActivateChaos` / `DeactivateChaos` / `SetChaosExpiry` compile to `OpUpdate` + `TargetChaosActivation` via `Apply`.
- `SimulateChaos` returns explained `hash-v1` decisions.
- `EmergencyDisableChaos` sets a store-level runtime inhibit bit that `Store.Swap` stamps onto every snapshot (apply cannot clear it). The emergency path CAS-stamps the current snapshot and does not roll back a concurrent apply. YAML `emergencyDisabled` still forces the bit on.
- `SIGUSR1` sets the runtime emergency bit. `labdns serve --chaos-disable` / `LABDNS_CHAOS_DISABLE=1/true/yes` set a separate startup lock that `Reset` and `EmergencyEnableChaos` cannot clear. `SIGUSR2` is ignored.
- CHA-002 executes the first-GA catalog in `internal/chaos/effects`: context-aware `fixed`/`uniform` delay (all four phases, budgets, cancel), RCODE/NODATA/EDE, TTL set/clamp/zero/jitter, alternate/omit/limit/shuffle/rotate, UDP drop/TC, TCP close/reset/hold, cache bypass/force-miss/expire/stale, upstream delay/unavailable/force/timeout/transport-error/failover/synthetic RCODE, and policy-scoped pressure. ADR 0007 malformed-wire effects are not implemented. Support matrix: [api/chaos/effects.json](https://github.com/hilather/go-lab-dns/blob/main/api/chaos/effects.json).

### Observability

- Catalog names, types, and labels are an operational compatibility surface. Forbidden default labels: raw QNAME, client IP, actor ID, idempotency key, free-form error text.
- Telemetry backpressure drops samples (`labdns_telemetry_dropped_total`) and never blocks DNS. Series per metric are capped at 256.
- `GET /v1/status` reports `ready`, `degraded`, and bounded warnings. Chaos (including emergency-disable) does not flip liveness, readiness, or degraded.
- `spec.observability.logQNAME` is the documented debug gate for QNAME/client in structured logs; default off.

### REST API

- stdlib `net/http` management server (`internal/control/rest`) exposes every first-GA capability on `/v1`. Default listen address is `:8080`.
- Generated OpenAPI 3.1: [api/openapi/v1.json](https://github.com/hilather/go-lab-dns/blob/main/api/openapi/v1.json).
- Errors use `capabilities.ProblemFrom` → `application/problem+json`. Wrong method on a registered path is `method_not_allowed` (405).
- Unauthenticated remote management is denied; `127.0.0.1` / `::1` may omit a bearer token. Health live/ready remain probe-accessible.
- `POST /v1/chaos:emergency-disable` exists and shares `app.Service`; packet-level chaos still runs on the DNS path independently of REST.
- `labdns serve` binds management HTTP (`rest.Server`, default `:8080`) together with DNS. `--management-listen=off` leaves it unbound.

### MCP API and protocol compatibility

- Pinned protocol **2026-07-28** (ADR 0006). `Mcp-Protocol-Version` is required and any other value is `unsupported_protocol_version`.
- Official Go SDK `github.com/modelcontextprotocol/go-sdk v1.7.0` behind `internal/control/mcp`. Streamable HTTP at `/mcp` is stateless (`StreamableHTTPOptions.Stateless=true`).
- Every first-GA MCP tool and `labdns://` resource is registered from `internal/capabilities`. Domain errors use `capabilities.JSONRPCFrom` so `data.code` matches REST.
- Four read-only prompts (`plan_dns_override`, `diagnose_resolution`, `design_chaos_experiment`, `convert_runtime_drift`) point at existing tools only.
- Stdio is a developer adapter (stderr logs, stdout protocol-clean) and is not required in the production image.

### Configuration and schema

- Strict v1alpha1 YAML/JSON decode in `internal/config` with unknown-field rejection, refuse-forward default-deny, materialized listen/TTL/CNAME defaults, canonical JSON/YAML export, and `sha256:` revision hashing.
- Published JSON Schema at [api/jsonschema/labdns.dev.v1alpha1.json](https://github.com/hilather/go-lab-dns/blob/main/api/jsonschema/labdns.dev.v1alpha1.json).
- `selector.timeBucket` must be `>= 1s`. Empty `clientGroups` is valid (local answers, no forwarding). Non-ASCII names, wildcard NS/DNAME, CNAME coexistence/loops, and self-forwarding are rejected.
- Config migration interface stub (`Migrator`); only `labdns.dev/v1alpha1` exists.

### Deployment and operations

- Required CI jobs now include `changelog`, `parity`, and `config-compat` in addition to format/lint/unit/race/fuzz-smoke/generated-file/documentation/security-scan/container-test. The `Release` workflow `tag-gate` refuses a tag when notes are incomplete, public-surface diffs are undocumented, generated files are stale, or a required CI job did not succeed on the exact commit. Capability-table diffs have their own review box (not the MCP box). `release-diff` writes the surface table to stdout so the uploaded `release-diff.txt` includes undocumented diffs. There is no `continue-on-error` or unbounded retry. Operator checklist: [docs/14-release-engineering.md](https://github.com/hilather/go-lab-dns/blob/main/docs/14-release-engineering.md).
- `labdns serve --config PATH` compiles bootstrap YAML, serves DNS on the configured listen address (default `:5353` UDP+TCP), and serves management HTTP on the configured address (default `:8080`). `--chaos-disable` / `LABDNS_CHAOS_DISABLE` arm a startup lock that YAML, reset, and emergency-enable cannot relax. `SIGTERM` is graceful (cancel delays, then listeners). `SIGUSR1` and `labdns chaos emergency-disable --pid-file` are the local runtime emergency path.
- `labdns serve --config PATH` compiles bootstrap YAML, serves DNS on the configured listen address (default `:5353` UDP+TCP), and serves management HTTP on the configured address (default `:8080`). `--chaos-disable` / `LABDNS_CHAOS_DISABLE` arm a startup lock that YAML, reset, and emergency-enable cannot relax. `SIGTERM` is graceful (cancel delays, then listeners). `SIGUSR1` and `labdns chaos emergency-disable --pid-file` are the local runtime emergency path. Serve loads `spec.management.auth` (`dev-loopback-unauth` or `bearer` + `secretRef`); a missing bearer file fails closed before accepting management.
- Runtime plan/apply/reset are process-local via `app.Service`. `expectedRevision` is required except privileged reset. Idempotency keys use a bounded in-memory LRU (default 256; `<=0` is not unlimited). Reset rereads the bootstrap mount and never writes it. Export is canonical YAML/JSON with no comments, plus bootstrap-to-runtime operations. Container recreation discards runtime drift.
- GitOps examples pin `ghcr.io/hilather/labdns@sha256:…`, bind management off the public DNS path, and keep tokens in env/Secret refs. `scripts/validate.sh` / `test-config.sh` / `deploy.sh` / `live-probe.sh` / `rollback.sh` have no bypass path.

### Observability

- None.

### Compatibility and migration

- Client interop fixtures (`testdata/interop/cases.json`) cover `dig`, Go `net.Resolver` (PreferGo), and raw UDP/TCP for exact, wildcard, NXDOMAIN/NODATA, CNAME, TTL, EDE, and UDP TC→TCP. Discovered client differences land as new cases.

### Known limitations

- Snapshot compilation, DNS wire, resolver, forwarder, and control-plane work have not started.
- Forwarder, cache, snapshot bootstrap serve, and control-plane work have not started.
- DNS listeners require a `dnsserver.Handler`; the process does not yet serve from a compiled snapshot (no `dnsquery` orchestrator yet).
- Snapshot bootstrap serve (`compiler.Compile`, `AccessIndex` fill, `cmd/labdns serve`) has not started; `dnsquery` is constructed in tests against a hand-built `snapshot.Store`.
- Plan/apply/export/reset and REST/MCP are not implemented; M1 serves only the compiled bootstrap snapshot.
- REST/MCP adapters are not implemented; `app.Service` is the HTTP-less mutation surface.
- MCP adapter is not implemented; REST `/v1` is available via `rest.Server` but not yet wired into `labdns serve` (DEP-001).
- `labdns chaos emergency-disable --pid-file` is not implemented until DEP-001.
- `make test-integration`, `make test-parity`, and `make test-container` fail closed until later PRs.
- YAML configuration decode, JSON Schema, snapshot compilation, resolver, forwarder, and control-plane work have not started.
- DNS listeners require a `dnsserver.Handler`; the process does not yet serve from a compiled snapshot.
- `make test-integration`, `make test-parity`, `make test-config-compat`, and `make test-container` fail closed until later PRs.
- `labdns serve` does not yet bind the management listener (REST `/v1` + MCP `/mcp`); DEP-001 wires `rest.Server` and `mcp.Server`.
- Full SEC-001 RBAC is not implemented; MCP uses the same loopback-unauth / remote-bearer stub as REST.
- GitOps/Compose/Kubernetes bearer examples land in GIT-001. Durable audit remains an optional hook, never fail-closed.
- `make test-integration` and `make test-container` fail closed until later PRs. `make test-parity` runs the REST/MCP registry and adapter suites.
- MCP adapter is not implemented; REST `/v1` is wired into `labdns serve`.
- `make test-parity` fails closed until MCP-001.
- Absolute QPS/latency numbers are recorded by benches, not gated in CI (hardware varies). Long soak is opt-in (`-soak=30m` / `LABDNS_SOAK_DURATION`).
- Durable audit remains an optional hook, never fail-closed. The GitOps template uses an all-zero digest placeholder until a released image is pinned.
- `make test-integration` fails closed until PERF-001.
- Durable list: [docs/known-limitations.md](https://github.com/hilather/go-lab-dns/blob/main/docs/known-limitations.md). Candidate notes: [docs/releases/v1.0.0-rc.1.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.0.0-rc.1.md). Evidence: [docs/releases/acceptance-evidence.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/acceptance-evidence.md).
- Absolute QPS/latency numbers are recorded by benches, not gated in CI (hardware varies). Long soak is opt-in (`-soak=30m` / `LABDNS_SOAK_DURATION`). The 24h pre-GA soak was not run.
- First public candidate: no predecessor tag; no image digest; tag-gate CI and SBOM/provenance wait for a human tag.
