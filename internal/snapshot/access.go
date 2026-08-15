package snapshot

import (
	"fmt"
	"net/netip"

	"github.com/hilather/go-lab-dns/internal/model"
)

// AccessIndex is the compiled CIDR-to-client-group structure.
//
// Zero value is valid and means "not compiled": Classify denies, and
// callers may fall back to walking Spec.Access. After CompileAccess
// returns, the index is immutable and Compiled reports true even when
// there are no groups (empty groups → every client is unknown).
type AccessIndex struct {
	// entries != nil marks a compiled index. A nil slice is the zero
	// value so dnsquery can still classify hand-built test snapshots.
	entries []accessEntry
}

type accessEntry struct {
	prefix       netip.Prefix
	group        model.ClientGroupID
	allowForward bool
}

// CompileAccess builds an AccessIndex from Spec.Access CIDRs.
//
// It is fail-closed on missing group IDs, empty CIDR lists, and unparsable
// prefixes even when config.Validate has already run. Nil state yields a
// compiled empty index (unknown clients, no forward).
func CompileAccess(st *model.State) (AccessIndex, error) {
	idx := AccessIndex{entries: []accessEntry{}}
	if st == nil {
		return idx, nil
	}
	for i, g := range st.Spec.Access.ClientGroups {
		if g.ID == "" {
			return AccessIndex{}, fmt.Errorf("access: clientGroups[%d] missing id", i)
		}
		if len(g.CIDRs) == 0 {
			return AccessIndex{}, fmt.Errorf("access: client group %q has no CIDRs", g.ID)
		}
		for j, c := range g.CIDRs {
			pfx, err := netip.ParsePrefix(c)
			if err != nil {
				return AccessIndex{}, fmt.Errorf("access: client group %q cidr[%d] %q: %w", g.ID, j, c, err)
			}
			idx.entries = append(idx.entries, accessEntry{
				prefix:       pfx,
				group:        g.ID,
				allowForward: g.AllowForward,
			})
		}
	}
	return idx, nil
}

// Compiled reports whether CompileAccess produced this index.
// The zero value is not compiled.
func (idx AccessIndex) Compiled() bool {
	return idx.entries != nil
}

// Classify returns the most-specific matching group and its AllowForward
// bit. Unknown or invalid addresses return ("", false) — deny forward.
// Longest prefix wins; equal-length ties keep the first configured group.
func (idx AccessIndex) Classify(addr netip.Addr) (model.ClientGroupID, bool) {
	if !addr.IsValid() || len(idx.entries) == 0 {
		return "", false
	}
	probe := addr
	if addr.Is4In6() {
		probe = addr.Unmap()
	}
	bestBits := -1
	var best accessEntry
	for _, e := range idx.entries {
		if !e.prefix.Contains(addr) && !e.prefix.Contains(probe) {
			continue
		}
		bits := e.prefix.Bits()
		if bits > bestBits {
			bestBits = bits
			best = e
		}
	}
	return best.group, best.allowForward
}
