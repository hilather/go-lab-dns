# Secrets

Never commit tokens. `spec.management.auth.secretRef` is a **file path**
inside the container (`/run/secrets/labdns-token`), not a value in Git.

Create a local token file for Compose:

```text
umask 077
openssl rand -hex 32 > secrets/labdns-token
```

Generate at least 32 bytes. Shorter tokens are deprecated: they still load
in this release and the next minor release refuses them.

Kubernetes: create an opaque Secret in the cluster and reference it from
the Deployment. Do not put the token in `dns.yaml` or `image.env`.

`labdns serve` with `auth.profile: bearer` fails closed if the file is
missing or empty. Under `bearer`, loopback (`127.0.0.1` / `::1`) and
remote management peers need `Authorization: Bearer` (MCP must send the
header; REST may use a session cookie created with a bearer). Health
live/ready stay unauthenticated. `dev-loopback-unauth` still lets
loopback omit the token.
