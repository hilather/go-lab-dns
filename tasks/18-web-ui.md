# UI-001–UI-004: Embedded operator web UI

Status: done (1.1.0 console; tag-gate pending)
Recommended owner: Control-plane / UI agent
Dependencies: API-001, MCP-001, SEC-001 (all done). Design: `docs/22-web-ui.md`, ADR 0008.
Exclusive ownership: `web/**`, `internal/web/**`, session code in `internal/auth`, SPA mount in `internal/control/rest`, UI bindings in `internal/capabilities`, Dockerfile Node stage, CI job `web`

This file is four sequential slices. All slices are complete. UI-002/UI-003 page-level overlap after UI-001 was allowed in `docs/22-web-ui.md`. Playwright (including apply-record and emergency-disable) is UI-004.

## Goal

Ship a reactive embedded console on the management listener so every public REST/MCP operator capability is completable in a browser, with the same authorization, plan/apply contract, and errors. The UI is a REST client only.

## Design references

- [x] `AGENTS.md`
- [x] `docs/22-web-ui.md`
- [x] `docs/05-control-plane-and-parity.md`
- [x] `docs/06-rest-api.md`
- [x] `docs/08-security-architecture.md`
- [x] `docs/10-testing-strategy.md`
- [x] ADR 0004, ADR 0008

## Explicit non-scope (all slices)

- UI calling MCP or implementing `internal/app` logic.
- SSE / WebSockets.
- HTTP Basic.
- Typed REST write routes that MCP does not have.
- OAuth/OIDC.
- Rewriting 1.0.0-rc.1 / rc.2 notes to claim a UI.

---

## UI-001 — Foundation

### Scope

- [x] `spec.ui.enabled` (default true) with config tests.
- [x] `POST/GET/DELETE /v1/session`, cookie `labdns_session`, header `X-LabDNS-CSRF`.
- [x] Registry rows: session + UI assets as `REST_ONLY_PROTOCOL`.
- [x] `web/` Vite + React 19.2 + TypeScript strict + Node 22.14.0 + npm lockfile.
- [x] `web/go.mod` fence; `internal/web` embed; `internal/web/stub` for `go test`.
- [x] `make web-test` and `make web-build` (fail closed, never no-op).
- [x] CI job `web` added to `RequiredCIJobs` and `.github/workflows/ci.yml`.
- [x] Dockerfile Node stage, image digest pin.
- [x] Login, shell chrome, dashboard that consumes `GET /v1/status` and version.
- [x] CSP and security headers from `docs/22-web-ui.md`.
- [x] Vite proxy for local `npm run dev` → `:8080`.
- [x] `cmd/labdns` wires `UI: web.Handler()`.

### Required tests

- [x] Session CSRF positive/negative; cookie flags; loopback unauth session create.
- [x] `ui.enabled: false` 404s `/` and still serves `/v1/state`.
- [x] No `Access-Control-Allow-*`; Origin policy unchanged.
- [x] Chaos-exempt path list includes `/` and `/v1/session`.
- [x] `web/src` unit: CSRF secret not written to `localStorage`/`sessionStorage`.
- [x] Generated OpenAPI includes session operations (`make verify-generated`).

### Acceptance

- Loopback browser can open `/`, log in, see status revision.
- Remote without bearer can load login HTML but cannot call `/v1` APIs.
- Production `go test ./...` does not require Node (stub embed).
- Image build fails if the Node stage did not emit `index.html` and hashed assets.

---

## UI-002 — Read console

Depends on UI-001.

### Scope

- [x] Generated OpenAPI TypeScript client (`openapi-fetch`); no hand-written DTO duplicates.
- [x] TanStack Query polling: status 2s; upstreams/cache/chaos status 5s; revision invalidation.
- [x] Pages: capabilities, schema, state (read + export download), zones/records (cursor pagination), resolve (query only), explain, forwarding/pools/upstream status, cache status, chaos status/policies, audit list/get, docs.
- [x] Scope-gated nav and disabled actions with named missing scope.

### Required tests

- [x] TS tests for query-key invalidation on revision change.
- [x] REST contract tests unchanged except new client generation.
- [x] Playwright matrix is UI-004 (`web/e2e`), not a UI-002 gate.

### Acceptance

- Every `PARITY_REQUIRED` **read** capability in `docs/22-web-ui.md` has a UI route.
- Viewer token can read and cannot submit mutations (buttons disabled + 403 if forced).

---

## UI-003 — Mutations

Depends on UI-001; pages not overlapping an in-flight UI-002 PR.

### Scope

- [x] Operation builder + YAML/JSON candidate editor.
- [x] Validate → plan → confirm apply (expected revision, idempotency UUID, reason, impact summary).
- [x] 409 revision conflict UX (refresh, do not overwrite).
- [x] Reset with typed confirmation.
- [x] Cache flush.
- [x] Chaos simulate, activate, deactivate, set expiry, emergency disable/enable with the documented privilege split.
- [x] Record/zone create-edit-delete only as operations into `/changes`.

### Required tests

- [x] Unit/Vitest for plan/apply, reset, flush, chaos mutations, and emergency confirm.
- [x] Playwright apply-a-record, export, reset, and emergency-disable cases landed in UI-004 (not a UI-003 gate).

### Acceptance

- Every `PARITY_REQUIRED` **mutating** capability is completable in the UI.
- No mutation skips plan when the REST capability is plan/apply.
- Audit events show `ui-session` / rest transport and a reason.

---

## UI-004 — Parity, a11y, ship

Depends on UI-002 and UI-003.

### Scope

- [x] Registry test: every `PARITY_REQUIRED` capability has a UI binding.
- [x] Playwright matrix covering the table in `docs/22-web-ui.md` (not screenshots-only).
- [x] Keyboard, labels, contrast, alert announcement tests.
- [x] GitOps/onboarding: how to open the UI on loopback `:8080`, token paste for remote, `ui.enabled`, `allowedOrigins`.
- [x] Update acceptance evidence, known limitations (UI no longer a residual non-goal for 1.1), CHANGELOG, 1.1.0 notes. Do not rewrite rc.1/rc.2.
- [x] `scripts/release-diff` accounts for session OpenAPI + `spec.ui` + capability UI map. Do not diff hashed `web/dist`.

### Required tests

- [x] `make test-parity` includes UI binding completeness.
- [x] `make web-test` and Playwright job green in CI (`make web-e2e`).
- [x] a11y unit tests from LabLDAP-style helpers (storage, labels).
- [x] Docs links and examples (`make test-docs`).

### Acceptance

- A reviewer can walk REST, MCP, and UI for one fixture change and see the same revision, diff, and audit code (`ui-session` / `rest`).
- Definition of done in `AGENTS.md` is satisfied for UI parity.
- 1.1.0 notes describe the UI completely: [docs/releases/v1.1.0.md](https://github.com/hilather/go-lab-dns/blob/main/docs/releases/v1.1.0.md).

---

## Security and safety review

- [x] Authentication/authorization impact reviewed
- [x] Input and resource limits reviewed
- [x] Secret/privacy impact reviewed (no token in Web Storage or logs)
- [x] Chaos caps/protected objects: UI cannot widen them; emergency path stays chaos-exempt

## Documentation updates

- [x] Normative: `docs/22-web-ui.md` Status Implemented; Last reviewed
- [x] REST session section in `docs/06-rest-api.md`
- [x] Parity table UI column in `docs/05-control-plane-and-parity.md`
- [x] Security session/CSP in `docs/08-security-architecture.md`
- [x] Operations and GitOps: how to use the console (`docs/11`, `docs/12`, `docs/13`, `examples/labdns-deploy`)
- [x] ADR 0008 accepted; not weakened
- [x] Unreleased changelog for operator-visible slices; candidate `docs/releases/v1.1.0.md` (tag not created in this change)

## CI requirements

- [x] All relevant local CI-equivalent commands pass
- [x] No skipped or weakened test
- [x] Generated files are current
- [x] CI failure, if encountered, was fixed and hardened
- [x] `web` is required; never optional

## Handoff

```text
Task ID: UI-001–UI-004
Commit/PR: execute-plan/87deccd2-pr-16-docs-operator-console-onboarding-and-110-release-n
Packages changed: docs, examples, CHANGELOG, docs/releases/v1.1.0.md, tasks/18, tasks/00-program-board, scripts/release-diff (omit hashed dist)
Tests added: release-diff omits hashed web assets
Commands run: make test-docs; make test-changelog
Compatibility impact: none beyond already-shipped spec.ui revision-hash (1.1.0)
Security impact: none; documents session/CSRF and token paste
Docs updated: docs/22 Status Implemented; onboarding in 11/12/13/examples; known-limitations; 19; 18 Phase 5
Release-note entry: docs/releases/v1.1.0.md (Unreleased changelog retained until tag)
Known limitations: no SSE/OAuth/TLS/shared sessions; HTTP cookie Secure false; stub embed in go test
```
