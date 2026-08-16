# Secrets

Never commit tokens. `spec.management.auth.secretRef` is a **file path**
inside the container (`/run/secrets/labdns-token`), not a value in Git.

Create a local token file for Compose:

```text
umask 077
printf 'replace-me-with-a-long-random-token\n' > secrets/labdns-token
```

Kubernetes: create an opaque Secret in the cluster and reference it from
the Deployment. Do not put the token in `dns.yaml` or `image.env`.

`labdns serve` with `auth.profile: bearer` fails closed if the file is
missing or empty. Loopback (`127.0.0.1` / `::1`) may still omit a
bearer; every remote management peer needs `Authorization: Bearer`.
