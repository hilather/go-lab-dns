# Deployment-repo operations

See also the application runbooks in the LabDNS repository
(`docs/13-operations-and-runbooks.md`).

## Routine

- `scripts/validate.sh` on every PR (required).
- `scripts/live-probe.sh` after deploy.
- Watch readiness, drift (`GET /v1/status`), chaos emergency bit, and
  `denied_forward` for unexpected unknown-client volume.

## Isolated management

Compose: `127.0.0.1:8080`. Kubernetes: NetworkPolicy + ClusterIP. Remote
calls need `Authorization: Bearer`. Health live/ready stay unauthenticated
so Docker HEALTHCHECK and kubelet work at the HTTP layer.

Kubernetes DNS Service uses `externalTrafficPolicy: Local` so
refuse-forward sees the real client IP. That pins the single replica to
the node that received the packet; use a DaemonSet/hostNetwork if you
need every node.

Management :8080 is allowed only from namespaces labeled
`labdns.dev/management=true` (see `k8s/management-namespace.yaml`). If
kubelet probes never become Ready on a policy-enforcing CNI, uncomment
the node-CIDR exception in `networkpolicy.yaml`.

## Chaos

Activate through REST/MCP with expiry. Emergency: `SIGUSR1`,
`labdns chaos emergency-disable --pid-file`, or
`POST /v1/chaos:emergency-disable`. Startup
`LABDNS_CHAOS_DISABLE=1` cannot be relaxed by YAML or API.

## Drift

Export (`dns_state_export`) and PR the canonical YAML. Recreating the
container or calling reset rereads the mounted file and **drops**
unsaved runtime mutations. That is the product (ADR 0003).
