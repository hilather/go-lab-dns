# GIT-001: Deployment Repository Examples and Probes

Status: done
Recommended owner: GitOps/platform agent
Dependencies: DEP-001, stable config schema, CLI verify command
Exclusive ownership: example deployment repository content and probe tooling integration

## Goal

Provide a separate, copyable deployment-repository pattern for Docker Compose and Kubernetes, with image pinning, desired state, policy checks, probes, drift promotion, and rollback.

## Work items

- [x] Create a reference deployment repository tree in examples or a separate template artifact.
- [x] Add Compose and Kubernetes examples with non-root/read-only settings and management isolation.
- [x] Pin image by digest and document update workflow.
- [x] Add environment-specific `dns.yaml` and `probes.yaml` examples.
- [x] Define probe schema and CLI verification behavior.
- [x] Add policy files for allowed upstreams, client networks, alternate addresses, protected names, and chaos caps.
- [x] Add validate, test-config, deploy, live-probe, and rollback scripts or documented equivalents.
- [x] Document runtime export to pull-request workflow.
- [x] Add CODEOWNERS and approval guidance for high-impact changes.
- [x] Ensure secrets are external references.

## Required tests

- [x] Positive and negative deployment config validation.
- [x] Image digest enforcement.
- [x] Probe execution for exact, wildcard, authoritative miss, overlay, upstream, and chaos simulation.
- [x] Policy rejection for broadened networks, unsafe chaos, and unapproved upstreams.
- [x] Container recreation resets runtime drift.
- [x] Rollback restores prior desired behavior.
- [x] Deployment CI failure path retains diagnostics and cannot be bypassed by scripts.
- [x] Regression test for every deployment tooling defect.

## Documentation updates

- [x] Finalize `docs/12-deployment-repository.md`.
- [x] Add environment onboarding and recovery instructions.
- [x] Update operations runbooks.
- [x] Add release-note entry for deployment template changes.

## Acceptance criteria

- A new lab can copy the template, set networks/domain/image digest, validate, deploy, and run probes.
- Runtime-to-Git promotion is documented and tested.
- No secret or mutable image tag is required.

## Handoff

Template: `examples/labdns-deploy` at this repository revision.
Supported application/config version: `labdns.dev/v1alpha1`.
Image: `ghcr.io/hilather/labdns` (digest-pinned in GitOps).
Probe API: `labdns.dev/probes/v1alpha1`.
Policy API: `labdns.dev/policy/v1alpha1`.
