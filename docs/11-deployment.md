# Container Deployment

Status: Proposed
Owners: Deployment, Operations, Security
Last reviewed: 2026-08-15 (DEP-001 CLI and hardened image)
Related ADRs: 0003

## Goals

- Reproducible, non-root, read-only container deployment.
- Ephemeral process state.
- Simple port 53 exposure.
- Safe management-network isolation.
- Predictable startup, reset, and shutdown.

## Container image

Image: **`ghcr.io/hilather/labdns`** (pin by digest in GitOps). The root `Dockerfile` is a multi-stage build (`golang:1.26-alpine` → `scratch`) that ships:

- One static `labdns` binary at `/labdns`.
- CA certificates for optional TLS upstreams.
- `LICENSE` (Apache-2.0) and OCI labels (`org.opencontainers.image.licenses=Apache-2.0`).
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
labdns verify --config=... --probes=...
labdns query --name=... --type=A [--server 127.0.0.1:5353] [--transport udp|tcp]
labdns healthcheck --url=http://127.0.0.1:8080/v1/health/ready
labdns chaos emergency-disable --pid-file=/tmp/labdns.pid
labdns version
```

| Interface | Meaning |
|---|---|
| `--config` | Bootstrap YAML/JSON path. Required for serve/validate/canonicalize/verify. The process never writes this file. |
| `--chaos-disable` | Startup inhibit. Same bit as `LABDNS_CHAOS_DISABLE=1/true/yes`. YAML and ordinary API cannot relax it. |
| `--dns-listen` | Override YAML DNS address. Empty uses YAML (`:5353`). |
| `--management-listen` | Override YAML management address. `off` / `none` / `-` leaves HTTP unbound. |
| `--shutdown-timeout` | Graceful deadline (default 5s). Cancels chaos delays, then DNS and management. |
| `--pid-file` | Written after both requested listeners bind. Removed on shutdown. |
| `LABDNS_CHAOS_DISABLE` | Only documented safety-related environment variable. No env var raises chaos safety caps. |

`labdns chaos emergency-disable --pid-file` sends **`SIGUSR1`** and does not call HTTP. `SIGTERM` / `SIGINT` are graceful shutdown. `SIGUSR2` is ignored.

## Compose example

```yaml
services:
  labdns:
    image: ghcr.io/hilather/labdns@sha256:REPLACE_WITH_DIGEST
    command: ["serve", "--config=/etc/labdns/config.yaml"]
    ports:
      - "53:5353/udp"
      - "53:5353/tcp"
      - "127.0.0.1:8080:8080/tcp"
    volumes:
      - "./dns.yaml:/etc/labdns/config.yaml:ro"
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
- Expose UDP and TCP port 53 through a service/load balancer that preserves required client identity or use node-local deployment.
- Isolate management ports with NetworkPolicy.
- Use a read-only root filesystem, seccomp, dropped capabilities, and a non-root security context.
- Set CPU, memory, and ephemeral-storage limits.
- Use disruption budgets only after multi-instance desired-state behavior is defined.

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
