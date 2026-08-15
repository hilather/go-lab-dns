// Package dnswire adapts github.com/miekg/dns to LabDNS model types.
//
// miekg/dns is pinned and may be imported only by this package. Public
// APIs accept and return model.Query, model.Result, and package-local
// Request/EDNS values — never dns.Msg or other library types.
//
// Parse never panics on malformed input. When a 12-byte header is
// present, callers can still emit a FORMERR; otherwise the datagram
// should be dropped.
package dnswire
