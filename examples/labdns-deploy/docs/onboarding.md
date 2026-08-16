# Environment onboarding

1. Copy `environments/main-lab` to a new directory name.
2. Change `metadata.name`, client CIDRs (must stay inside
   `policies/allowed-client-networks.yaml`), and zone names.
3. Pin `image.env` to a released digest and copy the same digest into
   `k8s/kustomization.yaml` when that directory exists.
4. Create the bearer token **outside Git** and point Compose
   `LABDNS_TOKEN_FILE` or the Kubernetes Secret at it.
5. Add probes for every name you expect humans or devices to use.
6. Run `scripts/test-config.sh <env>` until it is green.
7. Open a PR. CODEOWNERS must review `dns.yaml`, `image.env`, and `policies/`.
8. Merge and `scripts/deploy.sh <env> compose` (or `k8s`).

A new lab that skips digest pin, policy check, or probes is not onboarded.
