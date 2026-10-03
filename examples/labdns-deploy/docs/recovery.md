# Recovery

There is no runtime database. Recover from:

1. This Git repository (desired `dns.yaml` + `image.env`, plus the matching `k8s/kustomization.yaml` image pin when using Kubernetes).
2. The pinned image digest.
3. The out-of-band bearer Secret / token file.
4. External audit/telemetry if you attached a sink.

## Bad bootstrap

Do not restart a healthy process onto an invalid ConfigMap. `Reset` and
`labdns serve` fail closed and leave the previous snapshot (or do not
bind). Fix Git, re-run `scripts/test-config.sh`, then reset or recreate.

## Rollback

```text
# Durable
git revert <sha>
./scripts/deploy.sh main-lab compose

# Fast path (last successful deploy.sh snapshot)
./scripts/rollback.sh main-lab compose
```

Rollback restores prior **desired** behavior (records, pins, chaos caps).
Kubernetes snapshots restore both `image.env` and the matching Kustomize image pin. It does not replay discarded runtime experiments. A failed deployment leaves both successful snapshots intact: after successful A then B followed by failed C, `.last/` remains B and `.previous/` remains A, so rollback restores A.

## Recreate

`deploy.sh main-lab compose` uses `docker compose up --force-recreate`.
`deploy.sh main-lab k8s` applies manifests, explicitly restarts the Deployment,
and waits for rollout completion. Both start from the mounted YAML. Runtime drift is gone. Re-run `live-probe.sh`.
