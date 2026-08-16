# main-lab

Primary laboratory environment. Copy this directory, change networks and
the image digest, then run `../../scripts/validate.sh main-lab`.

| File | Role |
|---|---|
| `dns.yaml` | Desired DNS/chaos/management state (read-only mount) |
| `probes.yaml` | Offline + live verification probes |
| `image.env` | Digest-pinned `ghcr.io/hilather/labdns` |
| `compose.yaml` | Docker Compose: port 53, loopback `:8080`, token file |
| `k8s/` | Kubernetes: one replica, NetworkPolicy, Secret ref |

Create the bearer token before `compose up`:

```text
umask 077
printf 'dev-only-token\n' > ../../secrets/labdns-token
```

Kubernetes:

```text
kubectl create secret generic labdns-token \
  --namespace labdns-main-lab \
  --from-literal=token="$(cat ../../secrets/labdns-token)"
```

Do not commit that file. Remote `/v1` and `/mcp` need
`Authorization: Bearer`; loopback may omit it.
