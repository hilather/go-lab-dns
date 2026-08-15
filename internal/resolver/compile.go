package resolver

import (
	"fmt"
	"strings"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

const dnameTypeCode = 39

// Compile builds an immutable ZoneIndex from canonical state.
//
// It is fail-closed on zone-local invariants even when config.Validate has
// already run: wildcard NS, wildcard DNAME, CNAME coexistence, statically
// detectable CNAME loops, and CNAME at an authoritative apex. It does not
// require a fully populated Spec — a State with only Zones set is enough
// for unit tests.
func Compile(st *model.State) (snapshot.ZoneIndex, error) {
	idx := snapshot.ZoneIndex{ByID: map[model.ZoneID]*snapshot.ZoneData{}}
	if st == nil {
		return idx, nil
	}
	seenName := map[model.Name]model.ZoneID{}
	cnames := map[string]string{}
	for i := range st.Spec.Zones {
		z := st.Spec.Zones[i]
		if z.ID == "" {
			return snapshot.ZoneIndex{}, fmt.Errorf("%w: zone[%d] missing id", ErrInvalidZone, i)
		}
		if _, dup := idx.ByID[z.ID]; dup {
			return snapshot.ZoneIndex{}, fmt.Errorf("%w: duplicate zone id %q", ErrInvalidZone, z.ID)
		}
		origin := canonicalOwner(string(z.Name), "")
		if origin == "" {
			return snapshot.ZoneIndex{}, fmt.Errorf("%w: zone %q missing name", ErrInvalidZone, z.ID)
		}
		if prev, ok := seenName[origin]; ok {
			return snapshot.ZoneIndex{}, fmt.Errorf("%w: duplicate zone name %q (%s and %s)", ErrInvalidZone, origin, prev, z.ID)
		}
		seenName[origin] = z.ID
		zd, err := compileZone(z, origin, cnames)
		if err != nil {
			return snapshot.ZoneIndex{}, err
		}
		idx.ByID[z.ID] = zd
	}
	if err := detectCNAMELoops(cnames); err != nil {
		return snapshot.ZoneIndex{}, err
	}
	return idx, nil
}

func compileZone(z model.Zone, origin model.Name, cnames map[string]string) (*snapshot.ZoneData, error) {
	zd := &snapshot.ZoneData{
		ID:        z.ID,
		Name:      origin,
		Mode:      z.Mode,
		Exist:     map[model.Name]struct{}{},
		RRsets:    map[model.Name]map[model.RRType]snapshot.RRset{},
		Wildcards: map[model.Name]map[model.RRType]snapshot.RRset{},
	}
	markExist(zd, origin)
	if z.SOA != nil {
		soa := *z.SOA
		zd.SOA = &soa
	}
	if len(z.Nameservers) > 0 {
		ns := make([]model.Name, len(z.Nameservers))
		vals := make([]string, len(z.Nameservers))
		for i, n := range z.Nameservers {
			cn := canonicalOwner(string(n), origin)
			ns[i] = cn
			vals[i] = string(cn)
		}
		zd.Nameservers = ns
		if err := addRRset(zd, snapshot.RRset{
			Owner: origin,
			Type:  model.TypeNS,
			Class: model.ClassIN,
			Data:  vals,
		}, false); err != nil {
			return nil, err
		}
	}
	for i, r := range z.Records {
		if err := compileRecord(zd, origin, r, i, cnames); err != nil {
			return nil, err
		}
	}
	return zd, nil
}

func compileRecord(zd *snapshot.ZoneData, origin model.Name, r model.Record, idx int, cnames map[string]string) error {
	owner := canonicalOwner(r.Owner, origin)
	if owner == "" {
		return fmt.Errorf("%w: zone %q record[%d] missing owner", ErrInvalidZone, zd.ID, idx)
	}
	if !snapshot.InZone(owner, origin) {
		return fmt.Errorf("%w: zone %q record %q owner %q outside zone", ErrInvalidZone, zd.ID, r.ID, owner)
	}
	typ := model.RRType(strings.ToUpper(string(r.Type)))
	if r.GenericRDATA != nil && typ == "" {
		typ = model.RRType(fmt.Sprintf("TYPE%d", r.GenericRDATA.TypeCode))
	}
	if typ == "" {
		return fmt.Errorf("%w: zone %q record %q missing type", ErrInvalidZone, zd.ID, r.ID)
	}
	wild := snapshot.IsWildcardOwner(owner)
	if wild && typ == model.TypeNS {
		return fmt.Errorf("%w: wildcard NS is rejected (%s)", ErrInvalidZone, r.ID)
	}
	if wild && isDNAME(typ, r.GenericRDATA) {
		return fmt.Errorf("%w: wildcard DNAME is rejected (%s)", ErrInvalidZone, r.ID)
	}
	if typ == model.TypeCNAME && zd.Mode == model.ZoneModeAuthoritative && owner == origin {
		return fmt.Errorf("%w: CNAME is not allowed at the authoritative apex (%s)", ErrInvalidZone, r.ID)
	}
	if typ == model.TypeSOA && owner == origin && zd.SOA != nil {
		return fmt.Errorf("%w: duplicate apex SOA (%s)", ErrInvalidZone, r.ID)
	}

	var data []string
	if r.GenericRDATA != nil {
		data = []string{r.GenericRDATA.Presentation}
	} else {
		data = append([]string(nil), r.Values...)
		if typ == model.TypeCNAME || typ == model.TypeNS || typ == model.TypePTR {
			for i := range data {
				data[i] = string(canonicalOwner(data[i], origin))
			}
		}
	}
	if typ == model.TypeCNAME {
		if len(data) != 1 || data[0] == "" {
			return fmt.Errorf("%w: CNAME %q requires exactly one target", ErrInvalidZone, r.ID)
		}
		cnames[string(owner)] = data[0]
	}

	rr := snapshot.RRset{
		ID:    r.ID,
		Owner: owner,
		Type:  typ,
		Class: model.ClassIN,
		TTL:   r.TTL,
		Data:  data,
	}
	if err := addRRset(zd, rr, wild); err != nil {
		return err
	}
	return nil
}

func addRRset(zd *snapshot.ZoneData, rr snapshot.RRset, wild bool) error {
	markExist(zd, rr.Owner)
	byType := zd.RRsets[rr.Owner]
	if byType == nil {
		byType = map[model.RRType]snapshot.RRset{}
		zd.RRsets[rr.Owner] = byType
	}
	if _, exists := byType[rr.Type]; exists {
		return fmt.Errorf("%w: duplicate RRset %s %s", ErrInvalidZone, rr.Owner, rr.Type)
	}
	if rr.Type == model.TypeCNAME && len(byType) > 0 {
		return fmt.Errorf("%w: CNAME cannot coexist with other data at %s", ErrInvalidZone, rr.Owner)
	}
	if rr.Type != model.TypeCNAME {
		if _, has := byType[model.TypeCNAME]; has {
			return fmt.Errorf("%w: CNAME cannot coexist with other data at %s", ErrInvalidZone, rr.Owner)
		}
	}
	byType[rr.Type] = rr
	if wild {
		w := zd.Wildcards[rr.Owner]
		if w == nil {
			w = map[model.RRType]snapshot.RRset{}
			zd.Wildcards[rr.Owner] = w
		}
		w[rr.Type] = rr
	}
	return nil
}

func markExist(zd *snapshot.ZoneData, name model.Name) {
	cur := name
	for cur != "" {
		zd.Exist[cur] = struct{}{}
		if cur == zd.Name || cur == "." {
			return
		}
		next := snapshot.ParentName(cur)
		if next == cur {
			return
		}
		cur = next
	}
}

func detectCNAMELoops(cnames map[string]string) error {
	for start := range cnames {
		seen := map[string]bool{}
		cur := start
		for {
			next, ok := cnames[cur]
			if !ok {
				break
			}
			if seen[cur] {
				return fmt.Errorf("%w: CNAME loop involving %s", ErrInvalidZone, start)
			}
			seen[cur] = true
			cur = next
		}
	}
	return nil
}

func isDNAME(t model.RRType, g *model.GenericRDATA) bool {
	if strings.EqualFold(string(t), "DNAME") || t == "TYPE39" {
		return true
	}
	return g != nil && g.TypeCode == dnameTypeCode
}

func canonicalOwner(s string, origin model.Name) model.Name {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s == "@" {
		return origin
	}
	s = strings.ToLower(s)
	if strings.HasSuffix(s, ".") {
		return model.Name(s)
	}
	if origin == "" || origin == "." {
		return model.Name(s + ".")
	}
	return model.Name(s + "." + string(origin))
}
