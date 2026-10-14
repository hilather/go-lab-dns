# test-lab

Narrower client CIDR (`10.42.1.0/24`) and tighter chaos caps than
[main-lab](../main-lab/README.md). Compose publishes DNS on host 5353 so
it can sit beside main-lab. The process binds management on `:8080`; Compose publishes it on host `127.0.0.1:18080` — open
`http://127.0.0.1:18080/` and paste the bearer token. Kubernetes manifests
live under [main-lab/k8s](../main-lab/k8s/); copy and retarget the namespace.
