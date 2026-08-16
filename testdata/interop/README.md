# Client interoperability fixtures (PERF-001)

These fixtures are the regression surface for `dig`, the Go `net.Resolver`,
and a raw UDP/TCP client. Automated coverage lives in `internal/interop`.

## Cases

| ID | Name | Expectation |
|---|---|---|
| exact-a | `ns1.interop.lab.` A | NOERROR, AA, `10.42.0.53`, TTL 30s |
| wildcard | `foo.tools.interop.lab.` A | synthesized `10.42.0.20` |
| nxdomain | `no-such.interop.lab.` A | NXDOMAIN + SOA |
| nodata | `ns1.interop.lab.` AAAA | NOERROR, empty answer + SOA |
| empty-nonterminal | `tools.interop.lab.` A | NOERROR NODATA (empty non-terminal) |
| cname | `grafana.tools.interop.lab.` A | CNAME → `gateway.interop.lab.` + A |
| ttl | `ttl.interop.lab.` A | TTL 7s |
| ede | `fail.interop.lab.` A + EDNS | SERVFAIL + EDE 0 `lab-injected` |
| tc-tcp | `big.interop.lab.` TXT | UDP TC=1, TCP full TXT |

`cases.json` is the machine-readable copy of this table.

## Manual `dig`

Start LabDNS with `testdata/interop/config.yaml` (or let `go test ./internal/interop` bind an ephemeral port and print it).

```text
dig @127.0.0.1 -p 5353 +norecurse ns1.interop.lab. A
dig @127.0.0.1 -p 5353 +norecurse foo.tools.interop.lab. A
dig @127.0.0.1 -p 5353 +norecurse no-such.interop.lab. A
dig @127.0.0.1 -p 5353 +norecurse ns1.interop.lab. AAAA
dig @127.0.0.1 -p 5353 +norecurse grafana.tools.interop.lab. A
dig @127.0.0.1 -p 5353 +norecurse ttl.interop.lab. A
dig @127.0.0.1 -p 5353 +norecurse +edns=0 fail.interop.lab. A
dig @127.0.0.1 -p 5353 +norecurse +ignore +notcp big.interop.lab. TXT   # expect TC
dig @127.0.0.1 -p 5353 +norecurse +tcp big.interop.lab. TXT             # full TXT
```

Default `dig` retries UDP TC over TCP. `+ignore +notcp` isolates the truncated datagram.

## OS resolver

`internal/interop` uses `net.Resolver{PreferGo: true}` pointed at the test
listener. That is the portable OS-resolver stand-in; it does not rewrite
`/etc/resolv.conf`. glibc / systemd-resolved / Windows can be pointed at the
same listener in a lab and checked against `cases.json`.
