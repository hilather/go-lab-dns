package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

type idSet map[string]string // id -> first path

type catalog struct {
	zones         idSet
	records       idSet
	groups        idSet
	fwdPolicies   idSet
	pools         idSet
	upstreams     idSet
	chaosPolicies idSet
	cnames        map[string]string // owner FQDN -> target FQDN
	ownerTypes    map[string]map[model.RRType]string
	suffixes      idSet
	zoneCuts      idSet // canonical zone name -> path
	zoneNames     map[string]model.Name
	recordOwners  map[string]string // record ID -> owner FQDN
	recordZone    map[string]string // record ID -> zone ID
}

// Validate checks a (preferably normalized) state. It does not mutate st.
func Validate(st *model.State) error {
	if st == nil {
		return domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "state is nil"})
	}
	var vs []domainerr.FieldViolation
	cat := &catalog{
		zones:         idSet{},
		records:       idSet{},
		groups:        idSet{},
		fwdPolicies:   idSet{},
		pools:         idSet{},
		upstreams:     idSet{},
		chaosPolicies: idSet{},
		cnames:        map[string]string{},
		ownerTypes:    map[string]map[model.RRType]string{},
		suffixes:      idSet{},
		zoneCuts:      idSet{},
		zoneNames:     map[string]model.Name{},
		recordOwners:  map[string]string{},
		recordZone:    map[string]string{},
	}
	validateDocument(st, &vs)
	validateListeners(&st.Spec.Listeners, &vs)
	validateAccess(&st.Spec.Access, cat, &vs)
	validateDefaults(&st.Spec.Defaults, &vs)
	validateZones(st.Spec.Zones, cat, &vs)
	validateForwarding(&st.Spec.Forwarding, cat, &vs)
	validateCache(&st.Spec.Cache, &vs)
	validateChaos(&st.Spec.Chaos, cat, &vs)
	validateRecordChaosRefs(st, cat, &vs)
	validateManagement(&st.Spec.Management, &vs)
	validateCNAMELoops(cat, &vs)
	validateForwardLoops(&st.Spec.Listeners, &st.Spec.Forwarding, &vs)
	if len(vs) > 0 {
		return domainerr.ValidationFailed("Candidate state is invalid.", vs...)
	}
	return nil
}

func validateDocument(st *model.State, vs *[]domainerr.FieldViolation) {
	if st.APIVersion != model.APIVersionV1Alpha1 {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "apiVersion",
			Code:    violationUnsupportedVersion,
			Message: fmt.Sprintf("apiVersion must be %q", model.APIVersionV1Alpha1),
		})
	}
	if st.Kind != model.KindLabDNS {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "kind",
			Code:    violationInvalidValue,
			Message: fmt.Sprintf("kind must be %q", model.KindLabDNS),
		})
	}
	if strings.TrimSpace(st.Metadata.Name) == "" {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "metadata.name",
			Code:    violationRequired,
			Message: "metadata.name is required",
		})
	}
}

func validateListeners(l *model.ListenersSpec, vs *[]domainerr.FieldViolation) {
	if l.DNS.Address != "" {
		if _, _, err := net.SplitHostPort(l.DNS.Address); err != nil {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.listeners.dns.address",
				Code:    violationInvalidValue,
				Message: "DNS listen address must be host:port",
			})
		}
	}
	seen := map[model.Transport]bool{}
	for i, p := range l.DNS.Protocols {
		if !validTransport(p) {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    indexPath("spec.listeners.dns.protocols", i),
				Code:    violationInvalidTransport,
				Message: fmt.Sprintf("transport %q is not a v1alpha1 value (udp|tcp)", p),
			})
		}
		if seen[p] {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    indexPath("spec.listeners.dns.protocols", i),
				Code:    violationInvalidValue,
				Message: "duplicate protocol",
			})
		}
		seen[p] = true
	}
	if l.Management.Address != "" {
		if _, _, err := net.SplitHostPort(l.Management.Address); err != nil {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.listeners.management.address",
				Code:    violationInvalidValue,
				Message: "management listen address must be host:port",
			})
		}
	}
	if l.Management.RESTPath != "" && !strings.HasPrefix(l.Management.RESTPath, "/") {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.listeners.management.restPath",
			Code:    violationInvalidValue,
			Message: "restPath must start with /",
		})
	}
	if l.Management.MCPPath != "" && !strings.HasPrefix(l.Management.MCPPath, "/") {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.listeners.management.mcpPath",
			Code:    violationInvalidValue,
			Message: "mcpPath must start with /",
		})
	}
}

func validateAccess(a *model.AccessSpec, cat *catalog, vs *[]domainerr.FieldViolation) {
	if a.UnknownClient != "" && a.UnknownClient != model.UnknownClientRefuseForward {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.access.unknownClient",
			Code:    violationInvalidValue,
			Message: "unknownClient must be refuse-forward",
		})
	}
	for i, g := range a.ClientGroups {
		path := indexPath("spec.access.clientGroups", i)
		requireID(string(g.ID), path+".id", cat.groups, vs)
		if len(g.CIDRs) == 0 {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    path + ".cidrs",
				Code:    violationRequired,
				Message: "client group requires at least one CIDR",
			})
		}
		for ci, c := range g.CIDRs {
			if _, err := netip.ParsePrefix(c); err != nil {
				*vs = append(*vs, domainerr.FieldViolation{
					Path:    indexPath(path+".cidrs", ci),
					Code:    violationInvalidCIDR,
					Message: fmt.Sprintf("invalid CIDR %q", c),
				})
			}
		}
	}
}

func validateDefaults(d *model.DefaultsSpec, vs *[]domainerr.FieldViolation) {
	if d.TTL < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.defaults.ttl", Code: violationInvalidValue, Message: "ttl must be >= 0"})
	}
	if d.NegativeTTL < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.defaults.negativeTTL", Code: violationInvalidValue, Message: "negativeTTL must be >= 0"})
	}
	if d.CNAMEDepth < 1 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.defaults.cnameDepth", Code: violationInvalidValue, Message: "cnameDepth must be >= 1"})
	}
}

func validateZones(zones []model.Zone, cat *catalog, vs *[]domainerr.FieldViolation) {
	for i, z := range zones {
		path := indexPath("spec.zones", i)
		requireID(string(z.ID), path+".id", cat.zones, vs)
		if z.Name == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".name", Code: violationRequired, Message: "zone name is required"})
		} else if prev, ok := cat.zoneCuts[string(z.Name)]; ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".name", Code: violationDuplicateID, Message: "duplicate zone name (first at " + prev + ")"})
		} else {
			cat.zoneCuts[string(z.Name)] = path + ".name"
		}
		if string(z.ID) != "" {
			cat.zoneNames[string(z.ID)] = z.Name
		}
		if !validZoneMode(z.Mode) {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    path + ".mode",
				Code:    violationInvalidValue,
				Message: "mode must be authoritative or overlay",
			})
		}
		if z.Mode == model.ZoneModeAuthoritative && z.SOA == nil {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".soa", Code: violationRequired, Message: "authoritative zones require SOA"})
		}
		if z.SOA != nil {
			validateSOA(z.SOA, path+".soa", vs)
		}
		for ri, r := range z.Records {
			rp := indexPath(path+".records", ri)
			validateRecord(z, r, rp, cat, vs)
		}
	}
}

func validateSOA(soa *model.SOA, path string, vs *[]domainerr.FieldViolation) {
	if soa.Primary == "" {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".primary", Code: violationRequired, Message: "SOA primary is required"})
	}
	if soa.Administrator == "" {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".administrator", Code: violationRequired, Message: "SOA administrator is required"})
	}
	if soa.Serial != "auto" && soa.Serial != "" {
		if _, err := strconv.ParseUint(soa.Serial, 10, 32); err != nil {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".serial", Code: violationInvalidValue, Message: "serial must be \"auto\" or a 32-bit decimal"})
		}
	}
	if soa.Refresh < 0 || soa.Retry < 0 || soa.Expire < 0 || soa.Minimum < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationInvalidValue, Message: "SOA timings must be >= 0"})
	}
}

func validateRecord(z model.Zone, r model.Record, path string, cat *catalog, vs *[]domainerr.FieldViolation) {
	requireID(string(r.ID), path+".id", cat.records, vs)
	if string(r.ID) != "" {
		cat.recordOwners[string(r.ID)] = r.Owner
		cat.recordZone[string(r.ID)] = string(z.ID)
	}
	if r.Owner == "" {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".owner", Code: violationRequired, Message: "owner is required"})
	}
	if r.TTL < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".ttl", Code: violationInvalidValue, Message: "ttl must be >= 0"})
	}
	typ, generic := normalizeRRType(r)
	if typ == "" && r.GenericRDATA == nil {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".type", Code: violationInvalidType, Message: fmt.Sprintf("unsupported type %q", r.Type)})
	}
	if isWildcardOwner(r.Owner) {
		if typ == model.TypeNS {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".type", Code: violationWildcardNS, Message: "wildcard NS is rejected in v1alpha1"})
		}
		if isDNAME(typ, r.GenericRDATA) {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".type", Code: violationWildcardDNAME, Message: "wildcard DNAME is rejected"})
		}
	}
	if typ == model.TypeCNAME && z.Mode == model.ZoneModeAuthoritative && r.Owner == string(z.Name) {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".owner", Code: violationCNAMECoexist, Message: "CNAME is not allowed at the authoritative zone apex"})
	}
	if typ == model.TypeCNAME {
		if len(r.Values) != 1 {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".values", Code: violationInvalidValue, Message: "CNAME requires exactly one target"})
		} else {
			cat.cnames[r.Owner] = r.Values[0]
		}
	}
	if r.Owner != "" && typ != "" {
		ot, ok := cat.ownerTypes[r.Owner]
		if !ok {
			ot = map[model.RRType]string{}
			cat.ownerTypes[r.Owner] = ot
		}
		if prev, exists := ot[typ]; exists {
			*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationDupRRset, Message: "duplicate RRset at owner+type (first at " + prev + ")"})
		} else {
			ot[typ] = path
		}
		if typ == model.TypeCNAME && len(ot) > 1 {
			*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationCNAMECoexist, Message: "CNAME cannot coexist with other data at the same owner"})
		}
		if typ != model.TypeCNAME {
			if p, has := ot[model.TypeCNAME]; has {
				*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationCNAMECoexist, Message: "CNAME cannot coexist with other data at the same owner (CNAME at " + p + ")"})
			}
		}
	}
	validateRecordValues(r, typ, generic, path, vs)
	for i, ref := range r.ChaosPolicyRefs {
		if string(ref) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".chaosPolicyRefs", i), Code: violationEmptyID, Message: "empty chaos policy ref"})
		}
	}
}

func normalizeRRType(r model.Record) (model.RRType, bool) {
	if r.GenericRDATA != nil {
		return r.Type, true
	}
	t := model.RRType(strings.ToUpper(string(r.Type)))
	for _, known := range model.FirstGARRTypes {
		if t == known {
			return t, false
		}
	}
	if strings.HasPrefix(string(t), "TYPE") {
		return t, true
	}
	return "", false
}

func isDNAME(t model.RRType, g *model.GenericRDATA) bool {
	if strings.EqualFold(string(t), "DNAME") {
		return true
	}
	if t == "TYPE39" {
		return true
	}
	return g != nil && g.TypeCode == dnameTypeCode
}

func validateRecordValues(r model.Record, typ model.RRType, generic bool, path string, vs *[]domainerr.FieldViolation) {
	if generic {
		if r.GenericRDATA == nil {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".genericRdata", Code: violationRequired, Message: "TYPE<n> requires genericRdata"})
			return
		}
		if r.GenericRDATA.TypeCode == 0 {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".genericRdata.typeCode", Code: violationInvalidValue, Message: "typeCode must be non-zero"})
		}
		if strings.TrimSpace(r.GenericRDATA.Presentation) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".genericRdata.presentation", Code: violationRequired, Message: "presentation is required"})
		}
		return
	}
	if typ == "" {
		return
	}
	if len(r.Values) == 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".values", Code: violationRequired, Message: "values are required"})
		return
	}
	for i, v := range r.Values {
		vp := indexPath(path+".values", i)
		switch typ {
		case model.TypeA:
			ip := net.ParseIP(v)
			if ip == nil || ip.To4() == nil {
				*vs = append(*vs, domainerr.FieldViolation{Path: vp, Code: violationInvalidValue, Message: "A value must be an IPv4 address"})
			}
		case model.TypeAAAA:
			ip := net.ParseIP(v)
			if ip == nil || ip.To4() != nil {
				*vs = append(*vs, domainerr.FieldViolation{Path: vp, Code: violationInvalidValue, Message: "AAAA value must be an IPv6 address"})
			}
		case model.TypeMX:
			pref, name, ok := strings.Cut(strings.TrimSpace(v), " ")
			if !ok {
				*vs = append(*vs, domainerr.FieldViolation{Path: vp, Code: violationInvalidValue, Message: "MX value must be \"preference exchange\""})
				continue
			}
			if _, err := strconv.Atoi(pref); err != nil {
				*vs = append(*vs, domainerr.FieldViolation{Path: vp, Code: violationInvalidValue, Message: "MX preference must be an integer"})
			}
			if hasNonASCII(name) {
				*vs = append(*vs, domainerr.FieldViolation{Path: vp, Code: violationNonASCII, Message: "MX exchange is not ASCII"})
			}
		case model.TypeSRV:
			parts := strings.Fields(v)
			if len(parts) != 4 {
				*vs = append(*vs, domainerr.FieldViolation{Path: vp, Code: violationInvalidValue, Message: "SRV value must be \"priority weight port target\""})
			}
		}
	}
}

func validateForwarding(f *model.ForwardingSpec, cat *catalog, vs *[]domainerr.FieldViolation) {
	for i, p := range f.Pools {
		path := indexPath("spec.forwarding.pools", i)
		requireID(string(p.ID), path+".id", cat.pools, vs)
		if !validPoolStrategy(p.Strategy) {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".strategy", Code: violationInvalidValue, Message: "strategy must be ordered, round-robin, random, or health-aware"})
		}
		if len(p.Upstreams) == 0 {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".upstreams", Code: violationRequired, Message: "pool requires at least one upstream"})
		}
		for ui, u := range p.Upstreams {
			up := indexPath(path+".upstreams", ui)
			requireID(string(u.ID), up+".id", cat.upstreams, vs)
			if _, _, err := net.SplitHostPort(u.Endpoint); err != nil {
				*vs = append(*vs, domainerr.FieldViolation{Path: up + ".endpoint", Code: violationInvalidEndpoint, Message: "endpoint must be host:port"})
			}
			if !validTransport(u.Transport) {
				*vs = append(*vs, domainerr.FieldViolation{Path: up + ".transport", Code: violationInvalidTransport, Message: "transport must be udp or tcp"})
			}
		}
	}
	for i, p := range f.Policies {
		path := indexPath("spec.forwarding.policies", i)
		requireID(string(p.ID), path+".id", cat.fwdPolicies, vs)
		if p.Suffix == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".suffix", Code: violationRequired, Message: "suffix is required"})
		} else if prev, ok := cat.suffixes[string(p.Suffix)]; ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".suffix", Code: violationDuplicateID, Message: "duplicate forwarding suffix (first at " + prev + ")"})
		} else {
			cat.suffixes[string(p.Suffix)] = path + ".suffix"
		}
		if string(p.UpstreamPool) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".upstreamPool", Code: violationRequired, Message: "upstreamPool is required"})
		} else if _, ok := cat.pools[string(p.UpstreamPool)]; !ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".upstreamPool", Code: violationUnresolved, Message: "upstream pool " + string(p.UpstreamPool) + " does not exist"})
		}
		if p.Failover.Timeout < 0 {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".failover.timeout", Code: violationInvalidValue, Message: "timeout must be >= 0"})
		}
	}
}

func validateCache(c *model.CacheSpec, vs *[]domainerr.FieldViolation) {
	if c.Enabled && c.MaxEntries <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.cache.maxEntries", Code: violationInvalidValue, Message: "enabled cache requires maxEntries > 0"})
	}
	if c.MinimumTTL < 0 || c.MaximumTTL < 0 || c.MaximumNegativeTTL < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.cache", Code: violationInvalidValue, Message: "cache TTLs must be >= 0"})
	}
	if c.MinimumTTL > 0 && c.MaximumTTL > 0 && c.MinimumTTL > c.MaximumTTL {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.cache.minimumTTL", Code: violationInvalidValue, Message: "minimumTTL must be <= maximumTTL"})
	}
}

func validateChaos(c *model.ChaosSpec, cat *catalog, vs *[]domainerr.FieldViolation) {
	if c.Safety.MaxDelay < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.chaos.safety.maxDelay", Code: violationInvalidValue, Message: "maxDelay must be >= 0"})
	}
	if c.Safety.MaxDropProbability < 0 || c.Safety.MaxDropProbability > 1 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.chaos.safety.maxDropProbability", Code: violationInvalidValue, Message: "maxDropProbability must be in [0,1]"})
	}
	for i, cidr := range c.Safety.AllowedAddressCIDRs {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath("spec.chaos.safety.allowedAddressCIDRs", i), Code: violationInvalidCIDR, Message: "invalid CIDR"})
		}
	}
	requireClasses := map[model.SafetyClass]bool{}
	for _, cl := range c.Safety.RequireExpiryForSafetyClasses {
		requireClasses[cl] = true
	}
	protectedNames := map[model.Name]bool{}
	for _, n := range c.Safety.ProtectedNames {
		protectedNames[n] = true
	}
	protectedGroups := map[model.ClientGroupID]bool{}
	for _, g := range c.Safety.ProtectedClientGroups {
		protectedGroups[g] = true
		if _, ok := cat.groups[string(g)]; !ok && string(g) != "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: "spec.chaos.safety.protectedClientGroups", Code: violationUnresolved, Message: "protected client group " + string(g) + " does not exist"})
		}
	}
	for i, p := range c.Policies {
		path := indexPath("spec.chaos.policies", i)
		requireID(string(p.ID), path+".id", cat.chaosPolicies, vs)
		if strings.TrimSpace(p.Owner) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".owner", Code: violationRequired, Message: "policy owner is required"})
		}
		if strings.TrimSpace(p.Reason) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".reason", Code: violationRequired, Message: "policy reason is required"})
		}
		if !validSafetyClass(p.SafetyClass) {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".safetyClass", Code: violationInvalidValue, Message: "safetyClass must be low, medium, high, or unsafe-deferred"})
		}
		if p.Enabled && p.SafetyClass == model.SafetyClassUnsafeDeferred {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".safetyClass", Code: violationInvalidValue, Message: "unsafe-deferred policies cannot be enabled in the main process"})
		}
		if p.Enabled && (p.SafetyClass == model.SafetyClassHigh || requireClasses[p.SafetyClass]) && p.ExpiresAt == nil {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".expiresAt", Code: violationMissingExpiry, Message: "enabled high-impact policies require expiresAt"})
		}
		if p.Selector.Mode != "" && p.Selector.Mode != model.SelectorDeterministic && p.Selector.Mode != model.SelectorRandom {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".selector.mode", Code: violationInvalidValue, Message: "selector.mode must be deterministic or random"})
		}
		if p.Selector.Probability < 0 || p.Selector.Probability > 1 {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".selector.probability", Code: violationInvalidValue, Message: "probability must be in [0,1]"})
		}
		if p.Selector.TimeBucket != 0 && p.Selector.TimeBucket < MinTimeBucket {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    path + ".selector.timeBucket",
				Code:    violationTimeBucket,
				Message: "timeBucket must be >= 1s (hash-v1 encodes whole UTC seconds)",
			})
		}
		if p.Selector.EveryNth < 0 {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".selector.everyNth", Code: violationInvalidValue, Message: "everyNth must be >= 0"})
		}
		if p.Composition != "" && p.Composition != model.CompositionCompose && p.Composition != model.CompositionTerminal && p.Composition != model.CompositionExclusiveGroup {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".composition", Code: violationInvalidValue, Message: "composition must be compose, terminal, or exclusive-group"})
		}
		if p.Composition == model.CompositionExclusiveGroup && p.ExclusiveGroup == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".exclusiveGroup", Code: violationRequired, Message: "exclusive-group composition requires exclusiveGroup"})
		}
		validateChaosScope(p.Scope, path+".scope", cat, protectedNames, protectedGroups, vs)
		if len(p.Outcomes) == 0 {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".outcomes", Code: violationRequired, Message: "at least one outcome is required"})
		}
		for oi, o := range p.Outcomes {
			op := indexPath(path+".outcomes", oi)
			if o.ID == "" {
				*vs = append(*vs, domainerr.FieldViolation{Path: op + ".id", Code: violationEmptyID, Message: "outcome id is required"})
			}
			if o.Weight < 0 {
				*vs = append(*vs, domainerr.FieldViolation{Path: op + ".weight", Code: violationInvalidValue, Message: "weight must be >= 0"})
			}
			validateChaosActions(o.Actions, op+".actions", p, &c.Safety, vs)
		}
	}
}

func validateChaosScope(s model.ChaosScope, path string, cat *catalog, protectedNames map[model.Name]bool, protectedGroups map[model.ClientGroupID]bool, vs *[]domainerr.FieldViolation) {
	for i, id := range s.RecordIDs {
		if _, ok := cat.records[string(id)]; !ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".recordIds", i), Code: violationUnresolved, Message: "record " + string(id) + " does not exist"})
		} else if owner := cat.recordOwners[string(id)]; owner != "" && protectedNames[model.Name(owner)] {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".recordIds", i), Code: violationProtected, Message: "cannot target protected name " + owner})
		}
	}
	for i, id := range s.WildcardSourceIDs {
		if _, ok := cat.records[string(id)]; !ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".wildcardSourceIds", i), Code: violationUnresolved, Message: "record " + string(id) + " does not exist"})
		} else if owner := cat.recordOwners[string(id)]; owner != "" && protectedNames[model.Name(owner)] {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".wildcardSourceIds", i), Code: violationProtected, Message: "cannot target protected name " + owner})
		}
	}
	for i, id := range s.Zones {
		if _, ok := cat.zones[string(id)]; !ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".zones", i), Code: violationUnresolved, Message: "zone " + string(id) + " does not exist"})
		} else if zoneTargetsProtected(string(id), cat, protectedNames) {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".zones", i), Code: violationProtected, Message: "cannot target a zone that includes a protected name"})
		}
	}
	for i, id := range s.ForwardingIDs {
		if _, ok := cat.fwdPolicies[string(id)]; !ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".forwardingPolicyIds", i), Code: violationUnresolved, Message: "forwarding policy " + string(id) + " does not exist"})
		}
	}
	for i, id := range s.UpstreamPools {
		if _, ok := cat.pools[string(id)]; !ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".upstreamPools", i), Code: violationUnresolved, Message: "pool " + string(id) + " does not exist"})
		}
	}
	for i, id := range s.ClientGroups {
		if _, ok := cat.groups[string(id)]; !ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".clientGroups", i), Code: violationUnresolved, Message: "client group " + string(id) + " does not exist"})
		}
		if protectedGroups[id] {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".clientGroups", i), Code: violationProtected, Message: "cannot scope chaos to a protected client group"})
		}
	}
	for i, n := range s.Owners {
		if protectedNames[n] {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".owners", i), Code: violationProtected, Message: "cannot scope chaos to a protected name"})
		}
	}
	for i, t := range s.Transports {
		if !validTransport(t) {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".transports", i), Code: violationInvalidTransport, Message: "transport must be udp or tcp"})
		}
	}
}

func zoneTargetsProtected(zoneID string, cat *catalog, protectedNames map[model.Name]bool) bool {
	if name, ok := cat.zoneNames[zoneID]; ok && protectedNames[name] {
		return true
	}
	for recID, zid := range cat.recordZone {
		if zid != zoneID {
			continue
		}
		if owner := cat.recordOwners[recID]; owner != "" && protectedNames[model.Name(owner)] {
			return true
		}
	}
	return false
}

func validateChaosActions(actions []model.ChaosAction, path string, pol model.ChaosPolicy, safety *model.SafetySpec, vs *[]domainerr.FieldViolation) {
	lastRank := 0
	transportSeen := ""
	for i, a := range actions {
		ap := indexPath(path, i)
		if !validActionType(a.Type) {
			*vs = append(*vs, domainerr.FieldViolation{Path: ap + ".type", Code: violationInvalidValue, Message: fmt.Sprintf("unknown action type %q", a.Type)})
			continue
		}
		phase := a.Phase
		if phase == "" {
			phase = defaultPhase(a.Type)
		}
		rank := phaseRank(phase, a.Type)
		if rank == 0 && a.Phase != "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: ap + ".phase", Code: violationInvalidPhase, Message: fmt.Sprintf("invalid phase %q for %s", a.Phase, a.Type)})
		}
		if rank < lastRank {
			*vs = append(*vs, domainerr.FieldViolation{Path: ap + ".phase", Code: violationInvalidPhase, Message: "actions must be in phase order"})
		}
		if rank > lastRank {
			lastRank = rank
		}
		if isTransportAction(a.Type) {
			if transportSeen != "" {
				*vs = append(*vs, domainerr.FieldViolation{Path: ap + ".type", Code: violationConflict, Message: "drop/truncate/tcp-close/tcp-reset cannot be combined in one outcome"})
			}
			transportSeen = a.Type
		}
		if a.Type == model.ActionTruncate && transportsOnly(pol.Scope.Transports, model.TransportTCP) {
			*vs = append(*vs, domainerr.FieldViolation{Path: ap + ".type", Code: violationInvalidPhase, Message: "forced truncation is invalid on TCP-only scope"})
		}
		if (a.Type == model.ActionTCPClose || a.Type == model.ActionTCPReset) && transportsOnly(pol.Scope.Transports, model.TransportUDP) {
			*vs = append(*vs, domainerr.FieldViolation{Path: ap + ".type", Code: violationInvalidPhase, Message: a.Type + " is invalid on UDP-only scope"})
		}
		if a.Type == model.ActionDelay {
			validateDelay(a, ap, safety, vs)
		}
		if a.Type == model.ActionRCode {
			validateRCodeAction(a, ap, pol.SafetyClass, vs)
		}
		if a.Type == model.ActionAlternate {
			validateAlternate(a, ap, safety, vs)
		}
		if a.Type == model.ActionLimit && a.Limit < 0 {
			*vs = append(*vs, domainerr.FieldViolation{Path: ap + ".limit", Code: violationInvalidValue, Message: "limit must be >= 0"})
		}
		if a.Distribution != "" && a.Distribution != model.DistFixed && a.Distribution != model.DistUniform {
			*vs = append(*vs, domainerr.FieldViolation{Path: ap + ".distribution", Code: violationInvalidValue, Message: "distribution must be fixed or uniform"})
		}
	}
}

func validateDelay(a model.ChaosAction, path string, safety *model.SafetySpec, vs *[]domainerr.FieldViolation) {
	dist := a.Distribution
	if dist == "" {
		if a.Min != 0 || a.Max != 0 {
			dist = model.DistUniform
		} else {
			dist = model.DistFixed
		}
	}
	var maxd time.Duration
	switch dist {
	case model.DistFixed:
		if a.Duration < 0 {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".duration", Code: violationInvalidValue, Message: "duration must be >= 0"})
		}
		maxd = a.Duration
	case model.DistUniform:
		if a.Max < a.Min {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".max", Code: violationInvalidValue, Message: "uniform max must be >= min"})
		}
		maxd = a.Max
	}
	if safety != nil && safety.MaxDelay > 0 && maxd > safety.MaxDelay {
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationDelayCap, Message: "delay exceeds chaos.safety.maxDelay"})
	}
}

func validateRCodeAction(a model.ChaosAction, path string, class model.SafetyClass, vs *[]domainerr.FieldViolation) {
	v := strings.ToUpper(a.Value)
	switch model.RCode(v) {
	case model.RCodeServFail, model.RCodeRefused, model.RCodeNXDomain, model.RCodeNoError:
	case model.RCodeFormErr, model.RCodeNotImp:
		if class != model.SafetyClassMedium && class != model.SafetyClassHigh {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".value", Code: violationInvalidValue, Message: "FORMERR and NOTIMP require medium or high safetyClass"})
		}
	default:
		if v == "NODATA" {
			break
		}
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".value", Code: violationInvalidValue, Message: "unsupported injected RCODE"})
	}
}

func validateAlternate(a model.ChaosAction, path string, safety *model.SafetySpec, vs *[]domainerr.FieldViolation) {
	if len(a.Values) == 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: path + ".values", Code: violationRequired, Message: "alternate action requires values"})
		return
	}
	if safety == nil || len(safety.AllowedAddressCIDRs) == 0 {
		return
	}
	var nets []netip.Prefix
	for _, c := range safety.AllowedAddressCIDRs {
		p, err := netip.ParsePrefix(c)
		if err == nil {
			nets = append(nets, p)
		}
	}
	for i, v := range a.Values {
		addr, err := netip.ParseAddr(v)
		if err != nil {
			continue
		}
		ok := false
		for _, n := range nets {
			if n.Contains(addr) {
				ok = true
				break
			}
		}
		if !ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: indexPath(path+".values", i), Code: violationAltAddr, Message: "alternate address is outside allowedAddressCIDRs"})
		}
	}
}

func validateManagement(m *model.ManagementSpec, vs *[]domainerr.FieldViolation) {
	switch m.Auth.Profile {
	case "", model.AuthProfileDevLoopbackUnauth:
	case model.AuthProfileBearer:
		if strings.TrimSpace(m.Auth.SecretRef) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.auth.secretRef", Code: violationRequired, Message: "bearer profile requires secretRef"})
		}
	default:
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.auth.profile", Code: violationInvalidValue, Message: "profile must be dev-loopback-unauth or bearer"})
	}
	for i, origin := range m.AllowedOrigins {
		if !validHTTPOrigin(origin) {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    indexPath("spec.management.allowedOrigins", i),
				Code:    violationInvalidValue,
				Message: "origin must be an exact http(s)://host[:port] Origin with no path, query, or fragment",
			})
		}
	}
}

func validHTTPOrigin(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" || u.User != nil {
		return false
	}
	if u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.ForceQuery {
		return false
	}
	return s == u.Scheme+"://"+u.Host
}

func validateCNAMELoops(cat *catalog, vs *[]domainerr.FieldViolation) {
	for start := range cat.cnames {
		seen := map[string]bool{}
		cur := start
		for {
			next, ok := cat.cnames[cur]
			if !ok {
				break
			}
			if seen[cur] {
				*vs = append(*vs, domainerr.FieldViolation{Path: "spec.zones", Code: violationCNAMELoop, Message: "statically detectable CNAME loop involving " + start})
				break
			}
			seen[cur] = true
			cur = next
		}
	}
}

func validateForwardLoops(l *model.ListenersSpec, f *model.ForwardingSpec, vs *[]domainerr.FieldViolation) {
	listenHost, listenPort, err := net.SplitHostPort(l.DNS.Address)
	if err != nil {
		return
	}
	self := func(host, port string) bool {
		if port != listenPort {
			return false
		}
		h := strings.Trim(host, "[]")
		lh := strings.Trim(listenHost, "[]")
		if h == lh && lh != "" {
			return true
		}
		wildcardListen := lh == "" || lh == "0.0.0.0" || lh == "::" || lh == "*"
		if wildcardListen && isLoopbackOrUnspecified(h) {
			return true
		}
		return false
	}
	for pi, p := range f.Pools {
		for ui, u := range p.Upstreams {
			host, port, err := net.SplitHostPort(u.Endpoint)
			if err != nil {
				continue
			}
			if self(host, port) {
				*vs = append(*vs, domainerr.FieldViolation{
					Path:    indexPath(indexPath("spec.forwarding.pools", pi)+".upstreams", ui) + ".endpoint",
					Code:    violationForwardLoop,
					Message: "upstream endpoint points at this process (self-forward)",
				})
			}
		}
	}
}

func isLoopbackOrUnspecified(host string) bool {
	if host == "localhost" || host == "" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return ip.IsLoopback() || ip.IsUnspecified()
}

func requireID(id, path string, set idSet, vs *[]domainerr.FieldViolation) {
	if !validUserID(id) {
		code := violationEmptyID
		msg := "id is required"
		if id != "" {
			code = violationInvalidValue
			msg = "id must be non-empty printable ASCII without spaces"
		}
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: code, Message: msg})
		return
	}
	if prev, ok := set[id]; ok {
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationDuplicateID, Message: "duplicate id (first at " + prev + ")"})
		return
	}
	set[id] = path
}

func validTransport(t model.Transport) bool {
	return t == model.TransportUDP || t == model.TransportTCP
}

func validZoneMode(m model.ZoneMode) bool {
	return m == model.ZoneModeAuthoritative || m == model.ZoneModeOverlay
}

func validPoolStrategy(s model.PoolStrategy) bool {
	switch s {
	case model.StrategyOrdered, model.StrategyRoundRobin, model.StrategyRandom, model.StrategyHealthAware:
		return true
	default:
		return false
	}
}

func validSafetyClass(c model.SafetyClass) bool {
	switch c {
	case model.SafetyClassLow, model.SafetyClassMedium, model.SafetyClassHigh, model.SafetyClassUnsafeDeferred:
		return true
	default:
		return false
	}
}

func validActionType(t string) bool {
	switch t {
	case model.ActionDelay, model.ActionRCode, model.ActionDrop, model.ActionTruncate,
		model.ActionTCPClose, model.ActionTCPReset, model.ActionTTL, model.ActionAlternate,
		model.ActionOmit, model.ActionLimit, model.ActionShuffle, model.ActionRotate,
		model.ActionCache, model.ActionUpstream, model.ActionPressure:
		return true
	default:
		return false
	}
}

func isTransportAction(t string) bool {
	switch t {
	case model.ActionDrop, model.ActionTruncate, model.ActionTCPClose, model.ActionTCPReset:
		return true
	default:
		return false
	}
}

func defaultPhase(t string) string {
	switch t {
	case model.ActionDelay:
		return model.PhaseBeforeResponse
	case model.ActionRCode, model.ActionTTL, model.ActionAlternate, model.ActionOmit, model.ActionLimit, model.ActionShuffle, model.ActionRotate:
		return model.PhaseBeforeResponse
	case model.ActionCache, model.ActionPressure:
		return model.PhaseBeforeResolution
	case model.ActionUpstream:
		return model.PhaseBeforeUpstream
	case model.ActionDrop, model.ActionTruncate, model.ActionTCPClose, model.ActionTCPReset:
		return "transport"
	default:
		return ""
	}
}

func phaseRank(phase, actionType string) int {
	switch phase {
	case model.PhaseBeforeResolution:
		return 1
	case model.PhaseBeforeUpstream:
		return 2
	case model.PhaseAfterUpstream:
		return 3
	case model.PhaseBeforeResponse:
		return 4
	case "transport", "":
		if isTransportAction(actionType) {
			return 5
		}
		if phase == "" {
			return 0
		}
		return 0
	default:
		return 0
	}
}

func transportsOnly(have []model.Transport, only model.Transport) bool {
	if len(have) == 0 {
		return false
	}
	for _, t := range have {
		if t != only {
			return false
		}
	}
	return true
}

func validateRecordChaosRefs(st *model.State, cat *catalog, vs *[]domainerr.FieldViolation) {
	protected := map[model.Name]bool{}
	for _, n := range st.Spec.Chaos.Safety.ProtectedNames {
		protected[n] = true
	}
	for zi, z := range st.Spec.Zones {
		for ri, r := range z.Records {
			rp := indexPath(indexPath("spec.zones", zi)+".records", ri)
			if len(r.ChaosPolicyRefs) > 0 && protected[model.Name(r.Owner)] {
				*vs = append(*vs, domainerr.FieldViolation{
					Path:    rp + ".chaosPolicyRefs",
					Code:    violationProtected,
					Message: "cannot attach chaos to protected name " + r.Owner,
				})
			}
			for i, ref := range r.ChaosPolicyRefs {
				if _, ok := cat.chaosPolicies[string(ref)]; !ok && string(ref) != "" {
					*vs = append(*vs, domainerr.FieldViolation{
						Path:    indexPath(rp+".chaosPolicyRefs", i),
						Code:    violationUnresolved,
						Message: "chaos policy " + string(ref) + " does not exist",
					})
				}
			}
		}
	}
}
