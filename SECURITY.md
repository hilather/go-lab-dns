# Security Policy

## Security posture

LabDNS is a privileged network service. A DNS listener can become an amplification source or an unauthorized recursive resolver, and a management API can redirect laboratory traffic. Secure defaults are mandatory.

## Reporting vulnerabilities

Report security vulnerabilities through [GitHub private vulnerability reporting](https://github.com/hilather/go-lab-dns/security/advisories/new) on [`hilather/go-lab-dns`](https://github.com/hilather/go-lab-dns). Do not file vulnerabilities in the public issue tracker before coordinated disclosure.

## Minimum security requirements

- DNS recursion and forwarding are restricted to configured client networks.
- The management plane binds to loopback or a dedicated management network by default.
- REST and MCP share authentication, authorization, audit, and rate limiting.
- Chaos activation uses separate privileges from ordinary record editing.
- Secrets are never stored in bootstrap YAML committed to Git.
- Containers run as non-root with a read-only filesystem and no Linux capabilities.
- Dependencies, container images, and release artifacts are scanned.
- Release artifacts include provenance and an SBOM when the release pipeline supports them.
- Query names and client addresses are minimized or redacted in logs.
- Protected names and emergency controls cannot be modified by ordinary chaos permissions.

See `docs/08-security-architecture.md` for the complete design and `docs/20-threat-model.md` for threats and mitigations.
