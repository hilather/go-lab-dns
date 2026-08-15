# Deployment Repository Guide

Status: Proposed
Owners: Deployment, Platform
Last reviewed: 2026-08-15

## Purpose

The deployment repository is the durable source of truth for each environment. LabDNS runtime changes are ephemeral experiments until represented by a reviewed deployment-repository change.

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
    chaos-safety.yaml
  scripts/
    validate.sh
    test-config.sh
    deploy.sh
  docs/
    operations.md
    recovery.md
```

## Source-of-truth rules

- Pin images by digest.
- Mount `dns.yaml` read-only.
- Keep secrets outside Git.
- Require review for forwarding, protected names, management networks, and high-impact chaos changes.
- Store verification probes with desired state.
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

All checks are required. Pipeline failures are fixed and hardened; they are not bypassed.

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

## Probe format example

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

## Rollback

Rollback means reverting desired state or image pin in Git and redeploying. Runtime emergency rollback may deactivate a chaos policy or reset to bootstrap, but the deployment repository remains authoritative.

## Agent instructions for deployment changes

- Never commit secrets.
- Never broaden client CIDRs, upstreams, alternate-answer CIDRs, or chaos caps without explicit review.
- Always update probes for behavior changes.
- Always include release notes or environment change notes.
- Never deploy an image whose required CI or security checks failed.
- If deployment CI fails, fix and harden the pipeline before proceeding.

## Testing strategy

Test validation scripts, negative policy cases, image-digest enforcement, probe execution, rollback, and runtime-to-Git export compatibility.
