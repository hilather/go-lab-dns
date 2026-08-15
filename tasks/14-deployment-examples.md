# GIT-001: Deployment Repository Examples and Probes

Status: not-started
Recommended owner: GitOps/platform agent
Dependencies: DEP-001, stable config schema, CLI verify command
Exclusive ownership: example deployment repository content and probe tooling integration

## Goal

Provide a separate, copyable deployment-repository pattern for Docker Compose and Kubernetes, with image pinning, desired state, policy checks, probes, drift promotion, and rollback.

## Work items

- [ ] Create a reference deployment repository tree in examples or a separate template artifact.
- [ ] Add Compose and Kubernetes examples with non-root/read-only settings and management isolation.
- [ ] Pin image by digest and document update workflow.
- [ ] Add environment-specific `dns.yaml` and `probes.yaml` examples.
- [ ] Define probe schema and CLI verification behavior.
- [ ] Add policy files for allowed upstreams, client networks, alternate addresses, protected names, and chaos caps.
- [ ] Add validate, test-config, deploy, live-probe, and rollback scripts or documented equivalents.
- [ ] Document runtime export to pull-request workflow.
- [ ] Add CODEOWNERS and approval guidance for high-impact changes.
- [ ] Ensure secrets are external references.

## Required tests

- [ ] Positive and negative deployment config validation.
- [ ] Image digest enforcement.
- [ ] Probe execution for exact, wildcard, authoritative miss, overlay, upstream, and chaos simulation.
- [ ] Policy rejection for broadened networks, unsafe chaos, and unapproved upstreams.
- [ ] Container recreation resets runtime drift.
- [ ] Rollback restores prior desired behavior.
- [ ] Deployment CI failure path retains diagnostics and cannot be bypassed by scripts.
- [ ] Regression test for every deployment tooling defect.

## Documentation updates

- [ ] Finalize `docs/12-deployment-repository.md`.
- [ ] Add environment onboarding and recovery instructions.
- [ ] Update operations runbooks.
- [ ] Add release-note entry for deployment template changes.

## Acceptance criteria

- A new lab can copy the template, set networks/domain/image digest, validate, deploy, and run probes.
- Runtime-to-Git promotion is documented and tested.
- No secret or mutable image tag is required.

## Handoff

Provide the exact deployment template version and supported application/config versions.
