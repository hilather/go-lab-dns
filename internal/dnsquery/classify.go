package dnsquery

import (
	"net/netip"
	"strings"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// class is the per-query client + policy selection. ForwardingID is empty
// when the client must stay local (unknown, AllowForward=false, or no suffix).
type class struct {
	Group        model.ClientGroupID
	AllowForward bool
	ZoneID       model.ZoneID
	ForwardingID model.PolicyID
}

func classify(snap *snapshot.Snapshot, q model.Query) class {
	var out class
	if snap == nil {
		return out
	}
	out.Group, out.AllowForward = classifyClient(snap, q.Client)
	qname := canonicalName(string(q.Name))
	if zid, ok := snap.Zones.Select(qname); ok {
		out.ZoneID = zid
	}
	if out.AllowForward {
		if fid, ok := snap.Forwarding.Select(qname); ok {
			out.ForwardingID = fid
		}
	}
	return out
}

func classifyClient(snap *snapshot.Snapshot, addr netip.Addr) (model.ClientGroupID, bool) {
	if snap == nil || snap.Canonical == nil || !addr.IsValid() {
		return "", false
	}
	// AccessIndex fill is PR-07. Zero value means fall back to spec CIDRs.
	bestBits := -1
	var id model.ClientGroupID
	allow := false
	probe := addr
	if addr.Is4In6() {
		probe = addr.Unmap()
	}
	for _, g := range snap.Canonical.Spec.Access.ClientGroups {
		for _, c := range g.CIDRs {
			pfx, err := netip.ParsePrefix(c)
			if err != nil {
				continue
			}
			if !pfx.Contains(addr) && !pfx.Contains(probe) {
				continue
			}
			bits := pfx.Bits()
			if bits > bestBits {
				bestBits = bits
				id = g.ID
				allow = g.AllowForward
			}
		}
	}
	return id, allow
}

func canonicalName(s string) model.Name {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "."
	}
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return model.Name(s)
}
