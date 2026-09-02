package auth

import (
	"strings"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

// Protected is the compiled set of objects ordinary roles cannot mutate.
type Protected struct {
	Names        []model.Name
	ClientGroups []model.ClientGroupID
	RecordIDs    []model.RecordID
	ZoneIDs      []model.ZoneID
	Upstreams    []model.UpstreamID
}

// ProtectFrom reads SafetySpec plus records/zones whose owner is protected.
func ProtectFrom(st *model.State) Protected {
	if st == nil {
		return Protected{}
	}
	p := Protected{
		Names:        append([]model.Name(nil), st.Spec.Chaos.Safety.ProtectedNames...),
		ClientGroups: append([]model.ClientGroupID(nil), st.Spec.Chaos.Safety.ProtectedClientGroups...),
	}
	names := map[string]struct{}{}
	for _, n := range p.Names {
		k := canonName(string(n))
		if k != "" {
			names[k] = struct{}{}
		}
	}
	for _, z := range st.Spec.Zones {
		zname := canonName(string(z.Name))
		zoneHit := nameProtected(zname, names)
		if zoneHit {
			p.ZoneIDs = append(p.ZoneIDs, z.ID)
		}
		for _, r := range z.Records {
			owner := expandOwner(r.Owner, string(z.Name))
			if zoneHit || nameProtected(owner, names) {
				p.RecordIDs = append(p.RecordIDs, r.ID)
			}
		}
	}
	return p
}

func (p Protected) nameSet() map[string]struct{} {
	out := map[string]struct{}{}
	for _, n := range p.Names {
		if k := canonName(string(n)); k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

func (p Protected) hasRecord(id model.RecordID) bool {
	for _, r := range p.RecordIDs {
		if r == id {
			return true
		}
	}
	return false
}

func (p Protected) hasZone(id model.ZoneID) bool {
	for _, z := range p.ZoneIDs {
		if z == id {
			return true
		}
	}
	return false
}

func (p Protected) hasGroup(id model.ClientGroupID) bool {
	for _, g := range p.ClientGroups {
		if g == id {
			return true
		}
	}
	return false
}

func (p Protected) coversName(name string) bool {
	return nameProtected(canonName(name), p.nameSet()) || wildcardCoversProtected(name, p)
}

// wildcardCoversProtected reports whether a wildcard owner would synthesize
// answers for a protected name. Closest-encloser synthesis matches any name
// under the wildcard parent when no closer owner exists (docs/02).
func wildcardCoversProtected(owner string, prot Protected) bool {
	parent, ok := wildcardParent(owner)
	if !ok {
		return false
	}
	for _, n := range prot.Names {
		pn := canonName(string(n))
		if pn == "" || pn == parent {
			continue
		}
		if parent == "" || strings.HasSuffix(pn, "."+parent) {
			return true
		}
	}
	return false
}

func wildcardParent(owner string) (string, bool) {
	name := canonName(owner)
	if name == "*" {
		return "", true
	}
	if strings.HasPrefix(name, "*.") {
		return strings.TrimPrefix(name, "*."), true
	}
	return "", false
}

func canonName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, ".")
	return s
}

// expandOwner mirrors config.CanonicalName's relative-to-origin expansion
// without importing config (auth import DAG is model + domainerr only).
func expandOwner(owner, origin string) string {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return ""
	}
	if owner == "@" {
		return canonName(origin)
	}
	owner = strings.ToLower(owner)
	if strings.HasSuffix(owner, ".") {
		return canonName(owner)
	}
	orig := strings.ToLower(strings.TrimSpace(origin))
	if orig == "" || orig == "." {
		return canonName(owner + ".")
	}
	if !strings.HasSuffix(orig, ".") {
		orig += "."
	}
	return canonName(owner + "." + orig)
}

func findZone(st *model.State, id model.ZoneID) *model.Zone {
	if st == nil || id == "" {
		return nil
	}
	for i := range st.Spec.Zones {
		if st.Spec.Zones[i].ID == id {
			z := st.Spec.Zones[i]
			return &z
		}
	}
	return nil
}

func zoneHasProtectedRecord(z *model.Zone, prot Protected) bool {
	if z == nil {
		return false
	}
	if prot.hasZone(z.ID) || prot.coversName(string(z.Name)) {
		return true
	}
	for _, r := range z.Records {
		if prot.hasRecord(r.ID) {
			return true
		}
	}
	return false
}

func nameProtected(name string, names map[string]struct{}) bool {
	if name == "" || len(names) == 0 {
		return false
	}
	if _, ok := names[name]; ok {
		return true
	}
	// A protected zone name covers every name under that cut.
	for n := range names {
		if name == n || strings.HasSuffix(name, "."+n) {
			return true
		}
	}
	return false
}

func denyProtected(msg string) error {
	return domainerr.ProtectedObject(msg)
}
