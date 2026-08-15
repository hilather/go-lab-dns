package snapshot

import (
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// ZoneIndex is the compiled zone lookup structure. Zero value is valid
// (no zones; Select and Lookup miss).
//
// After resolver.Compile returns, the index is immutable. Callers must not
// mutate ByID or the ZoneData maps it holds.
type ZoneIndex struct {
	ByID map[model.ZoneID]*ZoneData
}

// ZoneData is one compiled zone: existence tree, RRsets, and wildcards.
type ZoneData struct {
	ID          model.ZoneID
	Name        model.Name
	Mode        model.ZoneMode
	SOA         *model.SOA
	Nameservers []model.Name
	// Exist holds every name that exists in the zone, including the apex
	// and empty non-terminals (ancestors of configured owners).
	Exist map[model.Name]struct{}
	// RRsets is owner → type → RRset, including literal wildcard owners.
	RRsets map[model.Name]map[model.RRType]RRset
	// Wildcards is the wildcard source owner (*.<encloser>.) → type → RRset.
	Wildcards map[model.Name]map[model.RRType]RRset
}

// RRset is one compiled resource-record set in presentation form.
type RRset struct {
	ID    model.RecordID
	Owner model.Name
	Type  model.RRType
	Class model.RRClass
	TTL   time.Duration
	Data  []string
}

// Lookup returns the compiled zone for id.
func (z ZoneIndex) Lookup(id model.ZoneID) (*ZoneData, bool) {
	if id == "" || z.ByID == nil {
		return nil, false
	}
	d, ok := z.ByID[id]
	return d, ok && d != nil
}

// Select returns the most-specific zone whose name is a suffix of qname.
// Resolve must not call Select; the orchestrator passes a pre-selected ID.
func (z ZoneIndex) Select(qname model.Name) (model.ZoneID, bool) {
	if z.ByID == nil {
		return "", false
	}
	var best model.ZoneID
	bestLen := -1
	for id, d := range z.ByID {
		if d == nil || !InZone(qname, d.Name) {
			continue
		}
		n := suffixRank(d.Name)
		if n > bestLen || (n == bestLen && string(id) < string(best)) {
			bestLen = n
			best = id
		}
	}
	return best, bestLen >= 0
}

// suffixRank is longer-is-more-specific. The root zone "." ranks 0.
func suffixRank(zone model.Name) int {
	if zone == "" || zone == "." {
		return 0
	}
	return len(string(zone))
}

// Contains reports whether name is the zone apex or a name below it.
func (d *ZoneData) Contains(name model.Name) bool {
	if d == nil {
		return false
	}
	return InZone(name, d.Name)
}

// HasName reports whether name exists in the zone (exact owner or empty
// non-terminal). A literal asterisk-label owner is an existing name.
func (d *ZoneData) HasName(name model.Name) bool {
	if d == nil || d.Exist == nil {
		return false
	}
	_, ok := d.Exist[name]
	return ok
}

// RRset returns the exact (non-synthesized) RRset at owner+type.
func (d *ZoneData) RRset(owner model.Name, t model.RRType) (RRset, bool) {
	if d == nil || d.RRsets == nil {
		return RRset{}, false
	}
	byType, ok := d.RRsets[owner]
	if !ok {
		return RRset{}, false
	}
	rr, ok := byType[t]
	if !ok {
		return RRset{}, false
	}
	return copyRRset(rr), true
}

// AllRRsets returns every exact RRset at owner, sorted by type mnemonic.
func (d *ZoneData) AllRRsets(owner model.Name) []RRset {
	if d == nil || d.RRsets == nil {
		return nil
	}
	byType, ok := d.RRsets[owner]
	if !ok || len(byType) == 0 {
		return nil
	}
	out := make([]RRset, 0, len(byType))
	for _, rr := range byType {
		out = append(out, copyRRset(rr))
	}
	sortRRsets(out)
	return out
}

// ClosestEncloser is the longest existing ancestor of name. name itself is
// not considered; the caller uses HasName first. The zone apex always
// encloses names inside the zone.
func (d *ZoneData) ClosestEncloser(name model.Name) model.Name {
	if d == nil {
		return ""
	}
	cur := ParentName(name)
	for cur != "" {
		if d.HasName(cur) {
			return cur
		}
		if cur == "." || cur == d.Name {
			return d.Name
		}
		next := ParentName(cur)
		if next == cur {
			return d.Name
		}
		cur = next
	}
	return d.Name
}

// Wildcard returns the wildcard source RRset at owner (*.<encloser>.) + type.
func (d *ZoneData) Wildcard(owner model.Name, t model.RRType) (RRset, bool) {
	if d == nil || d.Wildcards == nil {
		return RRset{}, false
	}
	byType, ok := d.Wildcards[owner]
	if !ok {
		return RRset{}, false
	}
	rr, ok := byType[t]
	if !ok {
		return RRset{}, false
	}
	return copyRRset(rr), true
}

// WildcardAll returns every wildcard source RRset at owner, sorted by type.
func (d *ZoneData) WildcardAll(owner model.Name) []RRset {
	if d == nil || d.Wildcards == nil {
		return nil
	}
	byType, ok := d.Wildcards[owner]
	if !ok || len(byType) == 0 {
		return nil
	}
	out := make([]RRset, 0, len(byType))
	for _, rr := range byType {
		out = append(out, copyRRset(rr))
	}
	sortRRsets(out)
	return out
}

// InZone reports whether name is zone or a descendant. The root zone "."
// contains every name. A suffix check requires a label boundary so
// notlab.example. is not inside lab.example.
func InZone(name, zone model.Name) bool {
	if zone == "" {
		return false
	}
	if zone == "." {
		return true
	}
	n, z := string(name), string(zone)
	if n == z {
		return true
	}
	return strings.HasSuffix(n, "."+z)
}

// ParentName returns the immediate parent of a FQDN, or "" for the root.
func ParentName(name model.Name) model.Name {
	s := string(name)
	if s == "" || s == "." {
		return ""
	}
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	rest := s[:len(s)-1]
	i := strings.IndexByte(rest, '.')
	if i < 0 {
		return "."
	}
	return model.Name(rest[i+1:] + ".")
}

// IsWildcardOwner reports a leftmost label of exactly "*".
func IsWildcardOwner(name model.Name) bool {
	s := string(name)
	return s == "*" || s == "*." || strings.HasPrefix(s, "*.")
}

// WildcardOwner is *.<encloser>. Root encloser "." yields "*.".
func WildcardOwner(encloser model.Name) model.Name {
	if encloser == "" || encloser == "." {
		return "*."
	}
	return model.Name("*." + string(encloser))
}

func copyRRset(rr RRset) RRset {
	if rr.Data != nil {
		rr.Data = append([]string(nil), rr.Data...)
	}
	return rr
}

func sortRRsets(rrs []RRset) {
	for i := 1; i < len(rrs); i++ {
		j := i
		for j > 0 && rrs[j].Type < rrs[j-1].Type {
			rrs[j], rrs[j-1] = rrs[j-1], rrs[j]
			j--
		}
	}
}
