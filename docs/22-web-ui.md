# Operator Web UI

Status: Implemented
Owners: Control Plane, REST, Security, UI
Last reviewed: 2026-08-19 (UI-004 onboarding, 1.1.0 notes; console complete)
Related ADRs: [0004](https://github.com/hilather/go-lab-dns/blob/main/docs/adr/0004-shared-capability-registry.md), [0008](https://github.com/hilather/go-lab-dns/blob/main/docs/adr/0008-embedded-operator-web-ui.md)

## Problem statement

REST and MCP already share one capability model. Humans in the mcp-integration-lab still need a browser console that can inspect and change the same state without curl, without a second API, and without drifting from agent workflows. Sibling appliances (LabLDAP, LabMail) embed a reactive SPA on the management listener. LabDNS must do the same, covering the full DNS/chaos/state surface.

## Goals

- Every public REST/MCP operator capability is reachable and completable in an embedded reactive web UI.
- The UI talks **REST only**, using generated OpenAPI types and the same authorization decision as MCP.
- Mutations always go through validate → plan → apply (or the typed chaos/cache/reset operations that already compile to `app.Service`), with expected revision, idempotency, reason, and impact summary.
- Browser auth uses HttpOnly session cookies and CSRF; bearer tokens never land in Web Storage.
- Same-origin embedding on the management listener; no permissive CORS.
- Chaos cannot affect UI assets, session endpoints, REST, MCP, health, or emergency disable.
- Parity tests fail if a new REST/MCP operator action has no UI path.

## Non-goals

- A third application adapter or UI-only business logic.
- Calling MCP from the browser.
- A public Internet administration product, OAuth/OIDC login, or shared session store.
- Typed REST CRUD write routes invented only for the UI (plan/apply remains the write path unless a later ADR adds typed writes to REST **and** MCP together).
- maildev-style WebSockets, a design-system kit, or unsafe-inline scripts.
- Server-sent events in the first UI increment (revision-based polling is enough; SSE is a documented follow-on).
- Changing DNS wire behavior, chaos effects, or GitOps persistence.
- Shipping REST/MCP-only operator workflows once the console exists (session/assets are REST_ONLY_PROTOCOL).

## Invariants

1. REST handlers, MCP handlers, and the UI never call each other. REST and MCP call `internal/app`. The UI calls REST.
2. Every public capability is declared in `internal/capabilities` with REST, MCP (unless `REST_ONLY_PROTOCOL`), and UI (unless `REST_ONLY_PROTOCOL` / `MCP_ONLY_PROTOCOL`) bindings.
3. The UI must not invent endpoints, scopes, defaults, or error codes.
4. Cookie-authenticated mutations require CSRF even on loopback HTTP.
5. DNS request handling does not depend on the UI. `spec.ui.enabled: false` 404s the SPA and keeps REST/MCP.
6. UI asset serving, session routes, health, and emergency-disable HTTP remain chaos-exempt management paths.
7. No secret, token, or CSRF secret in logs, metrics labels, export, or client storage besides the HttpOnly cookie and in-memory CSRF.

## Sibling alignment

Follow the mcp-integration-lab appliance console, not a generic admin template.

| Choice | LabDNS freeze | Source |
|---|---|---|
| Stack | React 19.2, TypeScript strict, Vite 8, Node **22.14.0** | LabLDAP, LabMail |
| Package manager | npm ≥10.9 with `package-lock.json` | LabMail (numbered-pack sibling) |
| Server state | TanStack Query; no duplicate global store of server data | LabLDAP |
| HTTP client | `openapi-fetch` against generated OpenAPI types | LabLDAP |
| Routing | `react-router` SPA | LabLDAP, LabMail |
| Source tree | `web/` (nested `web/go.mod` fence) | LabMail |
| Embed | `internal/web` `go:embed` of copied `web/dist`; committed stub in `internal/web/stub` | LabMail |
| Auth | `POST /v1/session` → HttpOnly cookie + CSRF JSON | LabLDAP, LabMail |
| CORS | None; same origin | All three |
| CSP | `script-src 'self'` only; no unsafe-inline script | LabLDAP |
| Markup | Semantic HTML; no large design-system dependency | LabLDAP OD-011 |

LabMail’s inbox is a narrow surface (messages). LabDNS is an operator console like LabLDAP: every capability gets a page or an explicit action on a page.

## Context and process model

```text
Lab clients ------> DNS :5353 (UDP/TCP)     [unchanged]
                       |
                       v
                 immutable snapshot
                       |
Management :8080 ------+------ REST /v1  ----+--> internal/app
  browser  GET / (SPA) |      MCP /mcp ------+
  same origin REST     |      session/CSRF
                       |
                 chaos must not reach this listener
```

The SPA is static files plus `index.html` fallback. All data and mutations are `/v1/...`. Vite dev proxies `/v1` and `/mcp` to `http://127.0.0.1:8080`.

## Package boundaries

```text
web/                         Vite app (Node; not imported by the parent Go module)
internal/web                 embed + SPA fallback + cache headers; no app import
internal/auth                session table, CSRF, cookie attributes
internal/control/rest        mounts /v1, session routes, UI assets, existing REST
internal/capabilities        UI bindings + dispositions
cmd/labdns                   wires embed; does not contain UI logic
```

Forbidden:

- `internal/web` importing `internal/app`, `internal/chaos`, `internal/dnsquery`, or MCP.
- `internal/dnsserver` / `internal/dnsquery` importing `internal/web`.
- Hand-written TypeScript DTO duplicates of OpenAPI schemas.
- UI calling `/mcp`.

`web/go.mod` is a nested-module fence so parent `go test ./...` does not walk `node_modules`. `//go:embed` cannot leave a module. `make web-build` writes **only** gitignored `web/dist/` and must not overwrite the tracked stub `internal/web/dist/index.html`. Production images copy `web/dist` into `internal/web/dist` in Docker after `COPY . .`. Local `go test` / `go run` embed the committed stub.

## Dispositions

Disposition is **derived** from `RESTOnly` (`RESTOnly` → `REST_ONLY_PROTOCOL`, otherwise `PARITY_REQUIRED`). It is not a stored catalog enum. Adapters and parity tests call `Capability.Disposition()`. A third value (`PARITY_DIFFERENT_BINDING`) requires an ADR before the registry grows a stored field.

| Disposition | UI requirement | Examples |
|---|---|---|
| `PARITY_REQUIRED` | Page or in-page action; `UI` is required | state, zones, records, resolve, explain, forwarding, cache, chaos, audit, plan/apply |
| `REST_ONLY_PROTOCOL` | No MCP tool. UI may *use* the route (session, assets) or *display* it (live/ready via dashboard) | health live/ready; session and SPA assets when added |
| `MCP_ONLY_PROTOCOL` | No UI row | `tools/list`, protocol negotiate (not in the current catalog) |
| `PARITY_DIFFERENT_BINDING` | Allowed only with an ADR | reserved for a later SSE vs MCP resource live-update |

Health live/ready stay process probes (not MCP tools). Their `UI` points at the dashboard (`Route: "/"`, `Action: "view"`). The dashboard displays ready/live from `GET /v1/status` and/or the probe routes; it does not reimplement readiness.

## Capability → UI map

Every current table row in [docs/05-control-plane-and-parity.md](05-control-plane-and-parity.md) maps as follows. New REST/MCP rows added later must extend this table in the same change. Registry `UIBinding.Route` / `Action` is the machine spelling; the notes column is operator UX.

Session create/get/delete and UI assets are `REST_ONLY_PROTOCOL` catalog rows (`session`, `ui.assets`) with `UI` omitted. `compileRoutes` skips `ui.assets` so `GET /` is the pre-auth SPA branch, not an authenticated registry dispatch.

| Capability | REST | UI route / action | Notes |
|---|---|---|---|
| Health live | `GET /v1/health/live` | `/` `view` (dashboard probe) | REST_ONLY |
| Health ready | `GET /v1/health/ready` | `/` `view` (dashboard probe) | REST_ONLY |
| Version | `GET /v1/version` | `/` `view` | Dashboard |
| Capabilities | `GET /v1/capabilities` | `/capabilities` `view` | Shell scope gating |
| Agent status | `GET /v1/status` | `/` `view` | Dashboard; poll source of truth for revision |
| Config schema | `GET /v1/schema/config` | `/schema` `view` | |
| Get state | `GET /v1/state` | `/state` `view` | |
| Validate | `POST /v1/state:validate` | `/state` `mutate` | Also used from `/changes` |
| Plan | `POST /v1/changes:plan` | `/changes` `mutate` | Always before apply |
| Apply | `POST /v1/changes:apply` | `/changes` `mutate` | expectedRevision + Idempotency-Key + reason |
| Export | `GET /v1/state:export` | `/state` `view` | Download YAML/JSON |
| Reset | `POST /v1/state:reset` | `/reset` `mutate` | Type confirmation; `dns.admin` |
| Zones list/get | `GET /v1/zones`, `GET /v1/zones/{zoneId}` | `/zones` `view` | Detail `/zones/:zoneId` |
| Records list/get | `GET /v1/zones/{zoneId}/records`, `.../{recordId}` | `/zones/:zoneId` `view` | Writes via plan/apply |
| Resolve | `POST /v1/resolve` | `/resolve` `view` | |
| Explain | `POST /v1/resolve:explain` | `/resolve` `view` | Explain tab |
| Forwarding | `GET /v1/forwarding/policies` | `/forwarding` `view` | |
| Pools | `GET /v1/upstream-pools` | `/forwarding` `view` | |
| Upstream status | `GET /v1/upstreams/status` | `/forwarding` `view` | Live poll; independent of snapshot revision |
| Cache status | `GET /v1/cache/status` | `/cache` `view` | Live poll; independent of snapshot revision |
| Cache flush | `POST /v1/cache:flush` | `/cache` `mutate` | `dns.admin`; not desired-state |
| Chaos status | `GET /v1/chaos/status` | `/chaos` `view` | Plus shell banner |
| Chaos policies | `GET /v1/chaos/policies`, `GET /v1/chaos/policies/{policyId}` | `/chaos` `view` | Detail `/chaos/:policyId` |
| Simulate | `POST /v1/chaos:simulate` | `/chaos` `view` | Side-effect free |
| Activate / deactivate | `POST ...:activate`, `:deactivate` | `/chaos/:policyId` `mutate` | Same scopes as REST/MCP |
| Set expiry | `POST ...:expire` | `/chaos/:policyId` `mutate` | |
| Emergency disable / enable | `POST /v1/chaos:emergency-disable`, `:emergency-enable` | `/` `mutate` | Shell emergency control |
| Audit list | `GET /v1/audit` | `/audit` `view` | |
| Audit get | `GET /v1/audit/{eventId}` | `/audit/:eventId` `view` | |
| Docs: DNS semantics | `GET /v1/docs/dns-semantics` | `/docs/:id` `view` | |
| Docs: chaos safety | `GET /v1/docs/chaos-safety` | `/docs/:id` `view` | |

`TestParityUIBindingsComplete` diffs this table (title, first `route` token, first `action` token) against the catalog and a frozen `map[ID]UIBinding`. Health live/ready (RESTOnly) also declare dashboard `UI`.

## Pages and UX

Shell (authenticated):

- Product name, revision short hash, ready/degraded, chaos emergency state.
- Nav: Overview, State, Changes, Zones, Resolve, Forwarding, Cache, Chaos, Audit, Schema, Docs, Capabilities.
- Scope-aware: actions the principal cannot perform are visible but disabled, with the missing scope named. Do not hide emergency disable from principals who have `dns.chaos.emergency`.
- Status uses a symbol plus a text label, not color alone.
- Problem+json `code` and `detail` are announced to assistive tech (`role="alert"`).

`/login`:

- Paste bearer token when the peer is not loopback-unauth.
- Loopback `dev-loopback-unauth`: allow “Continue as local administrator” which `POST /v1/session` with no bearer (same principal as today’s REST loopback).
- No HTTP Basic (LabDNS has no maildev Basic compat).
- After success, keep only the CSRF secret in process memory.

`/`:

- Status DTO: listeners, revisions, drift, cache summary, upstream summary, chaos summary, warnings.
- Live/ready widgets.
- Recent audit (if `dns.audit.read`).
- Quick actions gated by scope (plan, reset, cache flush, emergency disable).

`/state` and `/changes`:

- YAML/JSON editor for candidate spec **or** structured operation builder (add/update/delete zone, record, forwarding, chaos policy). Both compile to the same `Operation` list / candidate the REST API already accepts.
- Validate and plan are explicit buttons. Apply is disabled until a current plan for this revision exists.
- Show normalized diff, impact summary, warnings, required permissions.
- 409 `revision_conflict`: refresh status, discard the stale plan, do not overwrite.

`/reset`:

- Requires `dns.admin`, current revision, and typing the compiled metadata name (or the literal `RESET` if name is empty). Duplicate submits blocked while in flight.

`/zones`:

- Cursor pagination. Zone detail lists records with type/name/TTL. Record create/edit/delete enqueue operations and jump to `/changes` with those operations filled.

`/resolve`:

- Name, type, client group, transport, useCache, applyChaos (default off, matching REST).
- Side-by-side answer vs explain (matched zone, wildcard source, forwarder, cache, chaos decision).

`/forwarding` and `/cache`:

- Policies, pools, upstream health. Cache counters and bounded flush selector. Flush does not change desired state.

`/chaos`:

- Policy list with activation, safety class, expiry. Simulate form. Activate/deactivate/expiry with the same privilege split as REST (high-impact needs chaos-admin). Emergency disable is always on the shell for authorized principals.

`/audit`:

- Ring list, filters, event detail, retention notice. Secret-looking values stay redacted as the API returns them.

Treat all server strings as untrusted text. Do not render raw HTML. LabDNS has no HTML-mail preview.

## Reactive model

“Reactive” means the console tracks live process state without a full page reload.

First increment (required):

1. TanStack Query for every GET. Query keys include the resource and, where applicable, `status.revision`.
2. Poll `GET /v1/status` every **2s** while the document is visible (`document.visibilityState`). Pause when hidden.
3. When `revision` changes, invalidate state, zones, records, chaos policies, forwarding, schema, capabilities.
4. Poll `GET /v1/upstreams/status`, `GET /v1/cache/status`, and `GET /v1/chaos/status` every **5s** (they can change without a snapshot swap).
5. After a successful mutation, set the new revision from the response and invalidate immediately (do not wait for the next poll).
6. No WebSocket. No maildev live protocol.

Follow-on (not required to close UI-001–UI-004): `GET /v1/events/stream` SSE as `PARITY_DIFFERENT_BINDING` with an ADR. Do not add it in the first UI slices.

## Session and CSRF

New REST_ONLY capabilities (UI-001):

```text
POST   /v1/session
GET    /v1/session
DELETE /v1/session
```

Behavior (aligned with LabMail/LabLDAP, LabDNS cookie names):

- `POST /v1/session` with **no cookie header** authenticates with Identify (loopback unauth or `Authorization: Bearer`). CSRF is **omitted** on that first login only. A cookie that is present but unknown/expired, with no Bearer, is **401** and does not Identify (loopback Identify would mint administrator after idle expiry). On success: 32-byte random session ID in cookie `labdns_session` (`HttpOnly`, `SameSite=Lax`, `Secure` iff `r.TLS != nil`, `Path=/`, host-only) and a 32-byte CSRF secret in the JSON body (hex). `Cache-Control: no-store`.
- Cookie-present `POST /v1/session` **without** Bearer requires `X-LabDNS-CSRF` and **rotates** ID/CSRF for the **existing session Actor**. It must **not** call Identify (loopback Identify would upgrade a viewer UI session to administrator). Identity switch requires `Authorization: Bearer`.
- `Authorization: Bearer` wins: cookie and CSRF are ignored for that request. `POST /v1/session` with Bearer creates a **new** session for that token's Actor (old cookie session is left to expire or `DELETE`).
- Cookie-authenticated requests send `X-LabDNS-CSRF`. Required on every cookie non-GET, including cookie-present `POST /v1/session` and `DELETE /v1/session`. GET/HEAD never require CSRF.
- `GET /v1/session` returns the CSRF secret for a valid cookie (reload recovery). If GET fails, show `/login`.
- `DELETE /v1/session` revokes the server session (`Max-Age=0`) then the UI drops CSRF and Query cache.
- Session table is in-process memory (ADR 0003), max **256**, **12h sliding** TTL on any successful Lookup. At cap, reject **new** POST with `rate_limited` (429, detail `session table full`). Rotation does not consume a slot. Restart logs the operator in again. `ResetIfDigestChanged` exists for a later token-reread; 1.1.0 never calls it.
- Audit `transport` for cookie calls is `rest` with actor class `ui-session`. The underlying token ID remains the durable identity when a bearer was exchanged.
- MCP stays bearer-only. Cookies are ignored on `/mcp`.
- SPA `GET`/`HEAD` outside `/v1` and `/mcp` is served **before** authenticate. `ui.enabled: false` or a nil UI handler 404s those paths; they must **never** 401.

Loopback unauth does **not** skip CSRF for the SPA after login. Curl/SDK without Origin and without cookies still uses bearer or loopback as today.

## Configuration

Additive `labdns.dev/v1alpha1` fields (unknown-field rules unchanged for everything else):

```yaml
spec:
  ui:
    enabled: true          # default true when omitted
  management:
    auth:
      profile: dev-loopback-unauth
    allowedOrigins:        # exact http(s)://host[:port] Origin strings
      - https://dns-mgmt.lab.example
```

- Omitted `spec.ui` or omitted `enabled` materializes `true`, including documents with no `spec.access` block. Explicit `false` is preserved.
- `enabled: false`: management still serves REST and MCP; `GET /` and SPA fallback return 404. Session routes may stay mounted (harmless) or 404; freeze **404 for `/` and hashed assets only**, session routes remain so tests can still exercise CSRF independently.
- `spec.management.allowedOrigins` is an array of exact `http://` or `https://` Origins (`host` plus optional port, no path, query, or fragment). Omitted is empty. Invalid entries fail config validation (`invalid_value`).
- Target kind `ui` is an update-only singleton (`dns.admin`) so plan/apply/export cover `spec.ui` drift. `allowedOrigins` is replaced as part of the existing `management` target object.
- No `--disable-web` flag that disables management. `--management-listen=off` still unbinds the whole listener (REST, MCP, UI).
- Do not add UI listen addresses. The UI is not a second port.

Materialize the default in `internal/config` with valid/invalid/round-trip/compat fixtures.

## HTTP serving, CSP, headers

SPA assets are served from a **pre-auth** branch in `serveHTTP` after Origin check and security headers, for GET/HEAD whose path is not under `/v1` or `/mcp`. `/v1` and `/mcp` never fall through. `compileRoutes` omits `ui.assets`.

- Hashed assets: `Cache-Control: public, max-age=31536000, immutable`.
- `index.html`: `Cache-Control: no-store`, SPA fallback for non-file paths that are not `/v1`, `/mcp`, or well-known probes.
- Frozen CSP:

```text
Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
X-Frame-Options: DENY
```

- Still no `Access-Control-Allow-*`. OPTIONS is not a success path.
- Origin policy unchanged: missing Origin allowed (SDK); loopback Origin allowed; other Origins need `spec.management.allowedOrigins`. Same-origin UI on `:8080` is loopback or the management host the operator actually uses; if management is published as `https://dns-mgmt.lab.example`, that origin must be allowlisted in YAML as `spec.management.allowedOrigins`. Document this in GitOps examples.

## Mutation contract in the UI

The UI is not allowed a “save” that skips planning.

1. Build operations or a candidate spec.
2. `POST /v1/state:validate` and/or `POST /v1/changes:plan` with expected revision.
3. Render diff + impact summary + warnings.
4. On confirm, `POST /v1/changes:apply` with the same expected revision, a fresh idempotency key (UUID in component memory, reused only on retry of the same click), and a non-empty reason.
5. Typed chaos activate/deactivate/expire and cache flush and reset use their existing REST bodies, still with expected revision where the API requires it.

Do not add UI-only batch endpoints.

## Authorization

Same scopes as [docs/08-security-architecture.md](08-security-architecture.md). The shell loads `GET /v1/capabilities` and/or session principal scopes and gates buttons. A 403 is still handled (stale UI, reduced token). High-impact chaos activation in the UI must require the same chaos-admin split as REST/MCP.

## Failure modes

| Failure | Behavior |
|---|---|
| Session expired / CSRF mismatch | 401/403 → `/login`; do not loop |
| Revision conflict | Show current revision; require a new plan |
| Management TLS off on a non-loopback host | Cookie `Secure` false; document as lab-only; GitOps remains bearer on isolated network |
| `ui.enabled: false` | SPA 404; REST/MCP unchanged |
| Dist missing in a production build | Image build must fail (Dockerfile copies `web/dist`). Dev `go test` uses stub |
| Query poll while apply in flight | Do not clobber the in-progress plan form |
| Chaos delay on DNS | UI remains responsive; management is exempt |

## Security considerations

- XSS: text rendering only; CSP without unsafe-inline scripts; no `dangerouslySetInnerHTML`.
- CSRF: header required for cookie mutations.
- Token theft: token lives in the login form memory until POST, then discarded; cookie HttpOnly.
- Clickjacking: `frame-ancestors 'none'` and `X-Frame-Options: DENY`.
- DNS rebinding: existing Origin check.
- Audit: session create/delete and every mutation already audited; add `ui-session` actor class without logging the token or CSRF.
- Supply chain: pin Node image digest in the Dockerfile; `npm ci`; no un-reviewed UI dependencies. New npm packages need the same PR justification as Go modules.

## Observability

- Capability metrics already labeled by capability; session routes increment `labdns_capability_calls_total{capability="session",transport="rest"}`.
- No SPA request metrics in 1.1.0 (pre-auth branch never calls `observe()`; `ui.assets` is omitted from the live mux).
- Structured log `event=ui.session` with `actor_id` (token **id** or identity id such as `loopback`), never cookie, CSRF, or bearer value.

## Testing strategy

| Layer | What |
|---|---|
| Unit (Go) | Session create/delete/CSRF, cookie flags, `ui.enabled`, embed fallback, Origin unchanged, chaos-exempt paths include `/` and `/v1/session` |
| Unit (TS) | Query key/revision invalidation, operation-builder, a11y helpers, session-memory (no storage writes) |
| Contract | OpenAPI includes session; capability registry UI bindings complete |
| Parity | Existing REST/MCP goldens plus: UI binding present for every `PARITY_REQUIRED` row |
| E2E (Playwright) | Login; dashboard; plan/apply a record; resolve/explain; chaos simulate + emergency disable; reset confirmation; 403/disabled actions for viewer token |
| Security | Token not in `localStorage`/`sessionStorage`; CSP header; CSRF rejection; no CORS headers |
| Compat | Omitted `spec.ui` materializes `enabled: true`; `enabled: false` 404s SPA |

Playwright talks to a loopback `labdns serve` with `testdata/web/` (pack-sample plus viewer vs admin token sidecar). It is not a screenshot-only check. `make web-test` stays Vitest-only; the matrix is `make web-e2e` / `npm run test:e2e`.

CI (required — no optional job):

```text
make web-test
make web-build
npx playwright install --with-deps chromium
npm run test:e2e
```

GitHub job id `web`. Node 22.14.0 pinned. `make web-test` must fail closed if Node is missing in CI; local docs may skip with an explicit error, never a silent pass. Chromium is installed in the `web` job with `npx playwright install --with-deps chromium` (not a separate GitHub Action).

## Implementation plan

Work packages (task IDs UI-001–UI-004; all **done** — [tasks/18-web-ui.md](https://github.com/hilather/go-lab-dns/blob/main/tasks/18-web-ui.md)):

| Order | ID | Outcome | Status |
|---:|---|---|---|
| 18 | UI-001 | Session, embed, toolchain, login, shell, dashboard, CI `web` | done |
| 19 | UI-002 | All read pages + TanStack polling + generated client | done |
| 20 | UI-003 | All mutating workflows (plan/apply/reset/flush/chaos) | done |
| 21 | UI-004 | Playwright matrix, a11y, docs/examples, 1.1.0 notes | done |

Do not mark a later DNS/REST/MCP task complete if it adds a public operator capability without a UI action and a Playwright case.

Operator onboarding (loopback `:8080`, bearer paste, `spec.ui.enabled`, `spec.management.allowedOrigins`) lives in [docs/11-deployment.md](https://github.com/hilather/go-lab-dns/blob/main/docs/11-deployment.md), [docs/13-operations-and-runbooks.md](https://github.com/hilather/go-lab-dns/blob/main/docs/13-operations-and-runbooks.md), and [examples/labdns-deploy](https://github.com/hilather/go-lab-dns/blob/main/examples/labdns-deploy/README.md).

Dockerfile already has a Node stage **before** the Go build:

1. `npm ci` + `npm run build` in `web/`.
2. Copy `web/dist` into `internal/web/dist` **after** `COPY . .`.
3. Existing Go static build embeds dist. Image build fails if `index.html` or hashed `assets/` are missing.

Pin the Node base image by digest in the release Dockerfile, same policy as `golang:1.26.6-alpine`.

## Compatibility implications

- Additive OpenAPI paths `/v1/session` and static `/`.
- Additive config `spec.ui.enabled` with default true (omission keeps current listen behavior plus SPA).
- Cookie `labdns_session` is not an MCP concern.
- Hashed `web/dist` is **not** a `release-diff` surface (changes every build). Diff the capability UI map, OpenAPI session operations, and `spec.ui` schema. `internal/releasecontract.PublicSurfaces()` omits hashed assets.
- First UI increment is **1.1.0**. Candidate notes: [docs/releases/v1.1.0.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.1.0.md). rc.1/rc.2 notes were not rewritten to claim a UI.

## Open questions

Resolved for this design:

- SSE vs poll: **poll** for 1.1.0.
- Basic auth: **not** in LabDNS UI (no maildev swap constraint).
- Package manager: **npm**.
- Source directory: **`web/`**.
- Playwright browsers in CI: `npx playwright install --with-deps chromium` on job `web` (not a pinned install action).

Left closed for 1.1.0 (Node alpine digest is pinned in the Dockerfile). Follow-ons remain SSE, OAuth/OIDC, management TLS, and multi-replica sessions.

## Detailed design references

- Parity and registry: [docs/05-control-plane-and-parity.md](05-control-plane-and-parity.md)
- REST envelopes: [docs/06-rest-api.md](06-rest-api.md)
- Auth/RBAC: [docs/08-security-architecture.md](08-security-architecture.md)
- Tests: [docs/10-testing-strategy.md](10-testing-strategy.md)
