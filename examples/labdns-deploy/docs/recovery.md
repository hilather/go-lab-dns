# Recovery

There is no runtime database. Recover from:

1. This Git repository (desired `dns.yaml` + `image.env`).
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
It does not replay discarded runtime experiments.

## Recreate

`docker compose up --force-recreate` / a new Kubernetes ReplicaSet starts
from the mounted YAML. Runtime drift is gone. Re-run `live-probe.sh`.
