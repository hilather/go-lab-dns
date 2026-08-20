# ADR 0008: Embed an operator web UI as a REST client with REST/MCP/UI parity

Status: Accepted
Date: 2026-08-19

## Context

LabDNS already exposes one application service through REST `/v1` and MCP Streamable HTTP `/mcp` from a shared capability registry (ADR 0004). First GA deferred a web UI. Sibling mcp-integration-lab appliances ([LabLDAP](https://github.com/hilather/go-lab-ldap-mcp), [LabMail](https://github.com/hilather/go-lab-maildev)) ship an embedded React console on the management listener so a human can perform the same operator workflows as an agent.

Independent UIs that call ad-hoc endpoints, store bearer tokens in `localStorage`, or skip plan/apply would recreate the drift ADR 0004 exists to prevent. A second business-logic adapter beside REST and MCP would do the same.

## Decision

1. Ship an embedded operator SPA on the management HTTP listener, same process and same origin as REST.
2. The UI is a **REST client**, not a third application adapter. It must not call MCP, must not import `internal/app`, and must not implement plan/apply/reset/chaos/DNS logic.
3. Every public REST/MCP operator capability is reachable and completable in the UI in the same change that introduces it, except rows marked `REST_ONLY_PROTOCOL` or `MCP_ONLY_PROTOCOL` in [docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-dns/blob/main/docs/05-control-plane-and-parity.md) and [docs/22-web-ui.md](https://github.com/hilather/go-lab-dns/blob/main/docs/22-web-ui.md).
4. Browser authentication uses a short-lived in-memory session exchanged for a valid management principal (loopback unauth or bearer), an HttpOnly cookie, and a CSRF secret. Raw bearer tokens are never written to Web Storage, IndexedDB, URLs, or logs.
5. The capability registry gains a UI binding used by parity tests. REST/MCP/UI share authorization, domain types, revisions, idempotency, audit, and errors.

## Consequences

- Agents and humans share one mutation contract (validate, plan, apply, expected revision, reason).
- Session, static assets, and OpenAPI remain REST-only protocol surfaces (not MCP tools), matching LabMail.
- Same-origin embedding avoids CORS. Origin default-deny and CSP remain mandatory.
- The production image grows by a Node build stage and hashed static assets. `web/` is a nested-module fence so `go test ./...` does not walk `node_modules`.
- Chaos, DNS, and snapshot packages must not import the UI. UI asset serving is a protected management path.
- Product increment is **1.1.0**. rc.1/rc.2 notes must not be rewritten to claim a UI.

## Alternatives considered

- **No UI (status quo):** rejected. Operators in mcp-integration-lab need the same console pattern as LabLDAP/LabMail, and REST/MCP-only workflows drift.
- **Separate UI process or CDN:** rejected. Extra origin, CORS, and deployment surface. Lab appliances embed the SPA.
- **UI calls MCP from the browser:** rejected. MCP is agent transport; cookies/CSRF are HTTP session concerns; Origin and Streamable HTTP are a poor browser fit.
- **UI as a third `internal/app` adapter:** rejected. Duplicates REST and invites divergent validation.
- **Bearer token in `localStorage`:** rejected (LabLDAP/LabMail). XSS would exfiltrate admin credentials.

## Review triggers

Review this decision if the management listener is split from REST, if OAuth/OIDC replaces lab static bearer, if a multi-replica control plane needs shared sessions, or if a requirement appears for the UI to drive MCP directly.
