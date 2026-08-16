# LabDNS deployment repository template

Copy this tree into a **separate** Git repository. It is the durable source
of truth for each lab. Runtime plan/apply is ephemeral; only a reviewed
change here survives recreate or `state:reset`.

Application: LabDNS `labdns.dev/v1alpha1`  
Image: `ghcr.io/hilather/labdns` **pinned by digest**  
Auth: remote **bearer** via secret file / Secret ref; loopback may omit a token  
Probes: `labdns.dev/probes/v1alpha1`

## Layout

```text
labdns-deploy/
  README.md
  CODEOWNERS
  environments/
    main-lab/     dns.yaml probes.yaml compose.yaml image.env k8s/ README.md
    test-lab/     dns.yaml probes.yaml compose.yaml image.env README.md
  policies/       allowed upstreams, client networks, alternates, protected names, chaos caps
  scripts/        validate.sh test-config.sh deploy.sh live-probe.sh rollback.sh
  docs/           onboarding.md operations.md recovery.md
  secrets/        token file is local-only (gitignored)
```

## First lab

1. Copy this directory.
2. Edit `environments/main-lab/dns.yaml` (zones, CIDRs, upstreams).
3. Replace the all-zero digest in `image.env` **and** `k8s/kustomization.yaml`.
4. Write `secrets/labdns-token` (never commit it).
5. From the application repo, or with `labdns` on `PATH`:

```text
export LABDNS=/path/to/labdns
./scripts/validate.sh main-lab
./scripts/test-config.sh main-lab
./scripts/deploy.sh main-lab compose
./scripts/live-probe.sh main-lab 127.0.0.1:53 http://127.0.0.1:8080/v1/health/ready
```

`scripts/validate.sh` is schema + compile + probes + policy + digest pin.
There is no skip flag.

## Image digest update

1. Promote a `ghcr.io/hilather/labdns` digest whose required CI passed.
2. Set `LABDNS_IMAGE=ghcr.io/hilather/labdns@sha256:<64 hex>` in `image.env`.
3. Set the same digest on `k8s/kustomization.yaml` `images[].digest`.
4. Run `scripts/test-config.sh` and open a PR.

Mutable tags (`:latest`, `:v1`) fail `labdns verify --image`.

## Isolated management

Compose publishes `:8080` on `127.0.0.1` only. Kubernetes uses a ClusterIP
Service plus NetworkPolicy: DNS 5353 from the lab CIDR, management 8080
only from namespaces labeled `labdns.dev/management=true`.

`spec.management.auth.profile` is `bearer` with
`secretRef: /run/secrets/labdns-token`. The token is a mounted Secret /
file, not a Git field.

## Probes

Offline `labdns verify` uses the DNS orchestrator (not a management-only
lookup) so unknown-client behavior is real:

| Probe | Expectation |
|---|---|
| exact / wildcard | NOERROR, documented answers |
| authoritative miss | NXDOMAIN + AA |
| overlay hit | NOERROR, AA=0 |
| unknown client + local name | local RCODE, **RA=0**, no upstream dial |
| unknown client + forward-only name | **REFUSED**, **RA=0**, no upstream dial |
| chaos simulation | `slow-tools` triggered, no sleep |

Probes with `live: true` run only when `--server` is set (`live-probe.sh`).

## Runtime to Git

1. Plan/apply an experiment on the running process (expiry on chaos).
2. Verify with `resolve` / `explain` / probes.
3. `GET /v1/state:export` (canonical YAML + bootstrap-to-runtime ops).
4. Open a PR here with the export and updated probes.
5. Merge; `deploy.sh` recreates or operators `POST /v1/state:reset`.
6. Drift returns to false. Recreation always discards unsaved runtime state.

## Rollback

Preferred: revert the Git commit and `deploy.sh`.  
Fast path: `scripts/rollback.sh <env>` restores the previous successful
deploy snapshot (`.previous/`) and redeploys.

## Agent rules

- Never commit secrets or token files.
- Never broaden client CIDRs, upstreams, alternate-answer CIDRs, or chaos caps without review.
- Always update probes when behavior changes.
- Never deploy an image whose required CI failed.
- If a script or pipeline fails, fix and harden it; do not add `|| true`.
