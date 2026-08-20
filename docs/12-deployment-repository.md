# Deployment Repository Guide

Status: Normative (GIT-001)
Owners: Deployment, Platform
Last reviewed: 2026-08-19 (operator console :8080, ui.enabled, allowedOrigins)

## Purpose

The deployment repository is the durable source of truth for each environment. LabDNS runtime changes are ephemeral experiments until represented by a reviewed deployment-repository change.

The copyable template ships in this application repository at
[examples/labdns-deploy](https://github.com/hilather/go-lab-dns/blob/main/examples/labdns-deploy/README.md).
Copy that tree into a **separate** Git repo per lab. Application version:
`labdns.dev/v1alpha1`. Image: **`ghcr.io/hilather/labdns` pinned by digest**.

## Suggested layout

```text
labdns-deploy/
  README.md
  CODEOWNERS
  environments/
    main-lab/
      dns.yaml
      probes.yaml
      compose.yaml
      image.env
      k8s/
      README.md
    test-lab/
      dns.yaml
      probes.yaml
      compose.yaml
      image.env
      README.md
  policies/
    allowed-upstreams.yaml
    allowed-client-networks.yaml
    allowed-alternate-addresses.yaml
    protected-names.yaml
    chaos-safety.yaml
  scripts/
    validate.sh
    test-config.sh
    deploy.sh
    live-probe.sh
    rollback.sh
  docs/
    onboarding.md
    operations.md
    recovery.md
```

## Source-of-truth rules

- Pin images by digest (`name@sha256:<64 hex>`). Tags and `:latest` fail `labdns verify --image`. When `k8s/kustomization.yaml` exists, `--kustomize` must carry the **same** digest as `image.env`.
- Mount `dns.yaml` read-only at `/etc/labdns/config.yaml`.
- Keep secrets outside Git. GitOps auth is **bearer** via `secretRef` (file or Kubernetes Secret), never an inline token. Loopback (`127.0.0.1` / `::1`) may omit a bearer; every remote management peer must present one.
- Isolate management: Compose binds `:8080` to `127.0.0.1`; Kubernetes uses ClusterIP plus NetworkPolicy. Open the operator console at `http://127.0.0.1:8080/` after Compose up (paste the bearer token). `spec.ui.enabled: false` 404s the SPA only. A published management Origin must be listed in `spec.management.allowedOrigins` (exact `http(s)://host[:port]`; loopback Origin is already allowed).
- Require review for forwarding, protected names, management networks, and high-impact chaos changes (`CODEOWNERS`).
- Store verification probes with desired state (`labdns.dev/probes/v1alpha1`).
- Preserve historical release and schema compatibility through Git history.

## Pull request pipeline

```text
schema validation
 -> canonicalization and compile
 -> policy checks
 -> DNS semantic probes
 -> chaos safety checks
 -> REST/MCP schema compatibility if application version changes
 -> container smoke test
 -> review
 -> merge
 -> deploy
 -> live probes
```

All checks are required. Pipeline failures are fixed and hardened; they are not bypassed. The template scripts use `set -euo pipefail` and have no `|| true` on validate/verify.

`scripts/validate.sh` runs:

```text
labdns validate --config dns.yaml
labdns verify --config dns.yaml --probes probes.yaml \
  --policies policies/ --image-env image.env \
  [--kustomize environments/<env>/k8s/kustomization.yaml]
```

`labdns verify` compiles the document, checks digest pin and allowlists, then executes probes through the DNS orchestrator (`internal/dnsquery`) so refuse-forward and RA match the data plane. Probes with `live: true` run only when `--server` is set.

## Runtime-to-Git workflow

1. Agent plans an ephemeral runtime change.
2. Human or policy system approves activation.
3. Agent applies the change with expiry where appropriate.
4. Agent verifies using resolve/explain probes.
5. LabDNS exports canonical YAML and bootstrap-to-runtime operations.
6. Deployment automation creates a pull request in the deployment repository.
7. CI validates the exact desired state.
8. Reviewers merge.
9. Deployment recreates or resets LabDNS from Git state.
10. Drift returns to false.

Container recreation always discards unsaved runtime mutations (ADR 0003).

## Probe format example

Schema: [api/jsonschema/labdns.dev.probes.v1alpha1.json](https://github.com/hilather/go-lab-dns/blob/main/api/jsonschema/labdns.dev.probes.v1alpha1.json).

```yaml
apiVersion: labdns.dev/probes/v1alpha1
probes:
  - id: wildcard-tool
    query:
      name: random-name.tools.lab.example.net.
      type: A
      transport: udp
    expect:
      rcode: NOERROR
      answers:
        - 10.42.0.20

  - id: exact-over-wildcard
    query:
      name: grafana.tools.lab.example.net.
      type: CNAME
    expect:
      answers:
        - gateway.lab.example.net.

  - id: authoritative-miss
    query:
      name: missing.lab.example.net.
      type: A
    expect:
      rcode: NXDOMAIN

  - id: unknown-client-local
    query:
      name: ns1.lab.example.net.
      type: A
      client: 203.0.113.50
    expect:
      rcode: NOERROR
      ra: false
      noUpstream: true

  - id: unknown-client-forward-only
    query:
      name: only-forwarded.corp.example.net.
      type: A
      client: 203.0.113.50
    expect:
      rcode: REFUSED
      ra: false
      noUpstream: true

  - id: chaos-simulation
    simulateChaos: true
    query:
      name: alpha.tools.lab.example.net.
      type: A
      clientGroup: test-devices
    expect:
      matchedPolicy: slow-tools
      maximumDelay: 750ms
```

Unknown-client probes: a local name still succeeds with **RA=0**; a name that would only be forwarded is **REFUSED** with **RA=0** and no upstream dial. Overlay hits are AA=0. Authoritative misses do not fall through.

## Rollback

Rollback means reverting desired state or image pin in Git and redeploying. `scripts/rollback.sh` restores the previous successful `deploy.sh` snapshot (one generation) and redeploys. Runtime emergency rollback may deactivate a chaos policy or reset to bootstrap, but the deployment repository remains authoritative.

## Agent instructions for deployment changes

- Never commit secrets.
- Never broaden client CIDRs, upstreams, alternate-answer CIDRs, or chaos caps without explicit review.
- Always update probes for behavior changes.
- Always include release notes or environment change notes.
- Never deploy an image whose required CI or security checks failed.
- If deployment CI fails, fix and harden the pipeline before proceeding.

## Testing strategy

Test validation scripts, negative policy cases, image-digest enforcement, probe execution (exact, wildcard, authoritative miss, overlay, unknown-client refuse-forward, chaos simulation), rollback, container recreation reset-to-bootstrap, and runtime-to-Git export compatibility. A script or pipeline defect requires a regression test.

## Handoff

Template version is this repository revision. Supported application/config version: `labdns.dev/v1alpha1`. Image name: `ghcr.io/hilather/labdns`.
