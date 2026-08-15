# Standards and References

Status: Informational
Last reviewed: 2026-08-15

The implementation must pin exact dependency and protocol versions in the repository. This list identifies design inputs, not an instruction to track latest versions without review.

## DNS

- [RFC 1034 - Domain Names: Concepts and Facilities](https://www.rfc-editor.org/rfc/rfc1034.html)
- [RFC 1035 - Domain Names: Implementation and Specification](https://www.rfc-editor.org/rfc/rfc1035.html)
- [RFC 2308 - Negative Caching of DNS Queries](https://www.rfc-editor.org/rfc/rfc2308.html)
- [RFC 4592 - The Role of Wildcards in the Domain Name System](https://www.rfc-editor.org/rfc/rfc4592.html)
- [RFC 6891 - Extension Mechanisms for DNS (EDNS(0))](https://www.rfc-editor.org/rfc/rfc6891.html)
- [RFC 7766 - DNS Transport over TCP](https://www.rfc-editor.org/rfc/rfc7766.html)
- [RFC 7858 - DNS over TLS](https://www.rfc-editor.org/rfc/rfc7858.html)
- [RFC 8484 - DNS Queries over HTTPS](https://www.rfc-editor.org/rfc/rfc8484.html)
- [RFC 8375 - Special-Use Domain home.arpa](https://www.rfc-editor.org/rfc/rfc8375.html)
- [RFC 8914 - Extended DNS Errors](https://www.rfc-editor.org/rfc/rfc8914.html)
- [RFC 9460 - SVCB and HTTPS Resource Records](https://www.rfc-editor.org/rfc/rfc9460.html)

## MCP

- [MCP specification 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28)
- [MCP Streamable HTTP 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)
- [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)

The implementation baseline in this pack is MCP 2026-07-28. Newer revisions must be evaluated, pinned, conformance-tested, and documented before support is claimed.

## API and errors

- [OpenAPI Specification](https://spec.openapis.org/oas/)
- [RFC 9457 - Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457.html)

## Go libraries

The architecture recommends a DNS library behind `internal/dnswire` and the official MCP Go SDK behind `internal/control/mcp`. Pins: `github.com/miekg/dns v1.1.72` and `github.com/modelcontextprotocol/go-sdk v1.7.0` (MCP protocol 2026-07-28 only). Do not spread library-specific types through domain packages.
