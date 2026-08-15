# Container Deployment

Status: Proposed
Owners: Deployment, Operations, Security
Last reviewed: 2026-08-15
Related ADRs: 0003

## Goals

- Reproducible, non-root, read-only container deployment.
- Ephemeral process state.
- Simple port 53 exposure.
- Safe management-network isolation.
- Predictable startup, reset, and shutdown.

## Container image

Use a multi-stage build and a minimal runtime image. The runtime contains:

- One `labdns` binary.
- CA certificates only if required for TLS upstreams or management auth.
- License and build metadata.
- No shell unless a documented operational requirement justifies it.

Run as a numeric non-root UID. Listen on 5353 in the container and map host port 53.

## Compose example

```yaml
services:
  labdns:
    image: ghcr.io/example/labdns@sha256:REPLACE_WITH_DIGEST
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
