# Container Deployment

Status: Proposed
Owners: Deployment, Operations, Security
Last reviewed: 2026-08-19 (Dockerfile Node 22.14.0 stage for operator console)
Last reviewed: 2026-08-15 (PERF-001 capacity notes)
Last reviewed: 2026-08-15 (DEP-001 CLI; GIT-001 GitOps template)
Related ADRs: 0003

## Goals

- Reproducible, non-root, read-only container deployment.
- Ephemeral process state.
- Simple port 53 exposure.
- Safe management-network isolation.
- Predictable startup, reset, and shutdown.

## Container image

Image: **`ghcr.io/hilather/labdns`** (pin by digest in GitOps). The root `Dockerfile` is a multi-stage build (`node:22.14.0-alpine` digest-pinned → `golang:1.26.6-alpine` → `scratch`) that ships:

- One static `labdns` binary at `/labdns`.
- CA certificates for optional TLS upstreams.
- `LICENSE` (Apache-2.0) and OCI labels (`org.opencontainers.image.licenses=Apache-2.0`).
- Embedded operator-console assets from the Node stage (`web/dist` copied over `internal/web/dist` after `COPY . .`). The image build fails if `index.html` or hashed `assets/` are missing.
- No shell. `HEALTHCHECK` uses exec form `/labdns healthcheck`.

The image user is numeric **`65532:65532`**. Listen on 5353 in the container and map host port 53. Required runtime flags: `read_only: true`, `cap_drop: [ALL]`, `security_opt: [no-new-privileges:true]`, tmpfs `/tmp`.

SBOM and provenance attachments are REL-001 hooks; the image already carries license and source labels.

## CLI reference

```text
labdns serve --config=/etc/labdns/config.yaml [--chaos-disable]
             [--dns-listen ADDR] [--management-listen ADDR|off]
             [--shutdown-timeout 5s] [--pid-file /tmp/labdns.pid]
labdns validate --config=...
labdns canonicalize --config=... [--format yaml|json]
labdns verify --config=... --probes=... [--policies DIR] [--image REF|--image-env PATH] [--kustomize PATH] [--server HOST:PORT]
labdns query --name=... --type=A [--server 127.0.0.1:5353] [--transport udp|tcp]
labdns healthcheck --url=http://127.0.0.1:8080/v1/health/ready
labdns chaos emergency-disable --pid-file=/tmp/labdns.pid
labdns version
```

| Interface | Meaning |
|---|---|
| `--config` | Bootstrap YAML/JSON path. Required for serve/validate/canonicalize/verify. The process never writes this file. |
| `--chaos-disable` | Startup inhibit, same switch as `LABDNS_CHAOS_DISABLE=1/true/yes` (case-insensitive). Distinct from the runtime emergency bit: `Reset` and `emergency-enable` cannot relax it. Restart without the flag/env is the only off switch. |
| `--dns-listen` | Override YAML DNS address. Empty uses YAML (`:5353`). |
| `--management-listen` | Override YAML management address. `off` / `none` / `-` leaves HTTP unbound. |
| `--shutdown-timeout` | Graceful deadline (default 5s). Cancels chaos delays, then DNS and management. |
| `--pid-file` | Written after both requested listeners bind. Removed on shutdown. |
| `LABDNS_CHAOS_DISABLE` | Only documented safety-related environment variable. No env var raises chaos safety caps. |

`labdns chaos emergency-disable --pid-file` sends **`SIGUSR1`** and does not call HTTP. `SIGTERM` / `SIGINT` are graceful shutdown. `SIGUSR2` is ignored.

Copyable Compose, Kubernetes, policy, and probe files live in
[examples/labdns-deploy](https://github.com/hilather/go-lab-dns/blob/main/examples/labdns-deploy/README.md)
(GIT-001). Those examples use **bearer** `secretRef` (no token in Git) and
bind management to loopback or a NetworkPolicy-isolated ClusterIP.

## Compose example

The copyable file is
[examples/labdns-deploy/environments/main-lab/compose.yaml](https://github.com/hilather/go-lab-dns/blob/main/examples/labdns-deploy/environments/main-lab/compose.yaml).
It pins `${LABDNS_IMAGE:?…}` (digest from `image.env`), runs as `65532:65532`,
and mounts the bearer token file. Do not use a tag or an inline token.

```yaml
services:
  labdns:
    image: ${LABDNS_IMAGE:?set LABDNS_IMAGE from image.env}
    command: ["serve", "--config=/etc/labdns/config.yaml"]
    user: "65532:65532"
    ports:
      - "53:5353/udp"
      - "53:5353/tcp"
      - "127.0.0.1:8080:8080/tcp"
    volumes:
      - "./dns.yaml:/etc/labdns/config.yaml:ro"
      - "${LABDNS_TOKEN_FILE:-../../secrets/labdns-token}:/run/secrets/labdns-token:ro"
    read_only: true
    tmpfs:
      - /tmp
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/labdns", "healthcheck", "--url=http://127.0.0.1:8080/v1/health/ready"]
      interval: 10s
      timeout: 3s
      retries: 3
```

## Kubernetes guidance

- Use a ConfigMap or equivalent for non-secret bootstrap YAML.
- Use a Secret or workload identity for credentials.
- Run one replica for runtime mutation semantics in the initial release.
- Expose UDP and TCP port 53 through a Service with `externalTrafficPolicy: Local` (or a node-local DaemonSet / hostNetwork path) so refuse-forward classifies the real client IP. Default Cluster SNAT makes every query look like a node address.
- Isolate management ports with NetworkPolicy. Allow :8080 only from namespaces labeled `labdns.dev/management=true`. If kubelet probes never become Ready on a policy-enforcing CNI, add the node CIDR as documented in the template NetworkPolicy.
- Use a read-only root filesystem, seccomp, dropped capabilities, and a non-root security context.
- Set CPU, memory, and ephemeral-storage limits. First-GA starting point on the PERF-001 reference class (**2 vCPU / 4 GiB**): `cpu: 500m` request / `2` limit, `memory: 256Mi` request / `1Gi` limit. Raise memory before raising `spec.cache.maxEntries` or `spec.chaos.safety.maxConcurrentDelayed`.
- Use disruption budgets only after multi-instance desired-state behavior is defined.

## Capacity planning and safe default limits

These are the first-GA safe defaults. YAML may lower them; raising a chaos cap above the documented safety ceiling requires review.

| Limit | Default | Owner | Notes |
|---|---|---|---|
| `spec.chaos.safety.maxConcurrentDelayed` | 2000 (pack sample) | chaos budgets | Extra delayed queries skip sleep and still answer. Tested by `TestMaxDelayedConcurrency`. |
| `spec.chaos.safety.maxDelay` | 10s | chaos | Clamped per request. |
| DNS `MaxInflight` | 1024 | `dnsserver` | Surplus UDP datagrams are dropped (no queue blow-up). |
| DNS `MaxTCPConns` | 256 | `dnsserver` | Process-wide. |
| DNS `MaxTCPPerIP` | 16 | `dnsserver` | Per client address. |
| DNS `QueryTimeout` | 2s | `dnsserver` | Chaos delay may outlive this and still answer. |
| Cache `maxEntries` | 10000 (pack sample) | `internal/cache` | LRU; namespaced by snapshot revision. |
| Management `MaxConcurrent` | 256 | REST adapter | Control plane only. |

In-process local exact/wildcard/negative lookups are expected well under 1 ms on the reference class (see `go test ./benches -bench=.`). Forwarded QPS is dominated by upstream RTT. Chaos configured but not triggered (`probability: 0`) must stay on the same order as the exact path.

Do not run production with `maxConcurrentDelayed` unbounded (zero/negative is treated as unlimited by the budget table). Keep lab chaos expiry short.

## Startup and shutdown

Startup validates and compiles before reporting ready. Graceful shutdown:

1. Mark unready.
2. Stop accepting new management mutations.
3. Stop accepting new TCP connections.
4. Cancel outstanding bounded delays and upstream calls.
5. Complete or abandon requests within the shutdown deadline.
6. Flush bounded telemetry.
7. Exit.

## Runtime mutations and replicas

The initial release supports one mutable runtime replica. Multiple replicas may serve the same read-only desired state, but REST/MCP runtime mutations would diverge unless an external orchestrator applies the same change everywhere. Do not imply strong multi-replica runtime consistency.

## Emergency chaos disable

Deployments should set an environment-level maximum policy and provide an operational way to restart with chaos forcibly disabled. The startup override cannot be relaxed by YAML or ordinary API calls.

## Failure modes

- Invalid ConfigMap update: explicit reset fails and active state remains.
- Container recreation: runtime drift disappears and bootstrap state returns.
- Host port conflict: readiness never succeeds; deployment tooling reports it.
- Upstream outage: degraded status, not process crash.

## Observability

Expose build version, image digest if injected, config revision, process start time, readiness, and degraded reasons.

## Testing strategy

Run container tests for non-root UID, read-only filesystem, port mappings, reset, restart reset-to-bootstrap, signal handling, and emergency chaos disable.

## Compatibility implications

Container ports, CLI flags, environment variables, health paths, and filesystem paths are deployment interfaces and must follow deprecation policy.
