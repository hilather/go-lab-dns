// Package dnsquery orchestrates the DNS data-plane query path.
//
// cmd/labdns constructs the orchestrator and installs it as the DNS handler.
// This package must not be imported by the wire listener for domain logic.
//
// Classification happens before resolve: client group, zone, forwarding
// policy. A compiled AccessIndex is authoritative (including compiled-empty
// groups). An uncompiled zero AccessIndex falls back to
// snap.Canonical.Spec.Access CIDRs. Unknown and AllowForward=false groups
// receive local answers with RA=0 and are never forwarded. REFUSED is
// returned only when there is no local path.
//
// This v1 does not call chaos.Decide.
package dnsquery
