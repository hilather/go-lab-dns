package config

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

// Normalize returns a copy of st with defaults materialized and names
// canonicalized. The input is not mutated.
func Normalize(st *model.State) (*model.State, error) {
	if st == nil {
		return nil, domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "state is nil"})
	}
	out, err := cloneState(st)
	if err != nil {
		return nil, err
	}
	var vs []domainerr.FieldViolation
	materializeDefaults(&out.Spec)
	canonicalizeNames(&out.Spec, &vs)
	if len(vs) > 0 {
		return nil, domainerr.ValidationFailed("normalization failed", vs...)
	}
	return out, nil
}

func cloneState(st *model.State) (*model.State, error) {
	b, err := json.Marshal(st)
	if err != nil {
		return nil, domainerr.Internal("clone marshal: " + err.Error())
	}
	var out model.State
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, domainerr.Internal("clone unmarshal: " + err.Error())
	}
	return &out, nil
}

func materializeDefaults(sp *model.Spec) {
	if sp.Access.UnknownClient == "" {
		sp.Access.UnknownClient = model.UnknownClientRefuseForward
	}
	if sp.Access.ClientGroups == nil {
		sp.Access.ClientGroups = []model.ClientGroup{}
	}
	if sp.Listeners.DNS.Address == "" {
		sp.Listeners.DNS.Address = DefaultDNSAddress
	}
	if len(sp.Listeners.DNS.Protocols) == 0 {
		sp.Listeners.DNS.Protocols = []model.Transport{model.TransportUDP, model.TransportTCP}
	}
	if sp.Listeners.Management.Address == "" {
		sp.Listeners.Management.Address = DefaultMgmtAddress
	}
	if sp.Listeners.Management.RESTPath == "" {
		sp.Listeners.Management.RESTPath = DefaultRESTPath
	}
	if sp.Listeners.Management.MCPPath == "" {
		sp.Listeners.Management.MCPPath = DefaultMCPPath
	}
	if sp.Defaults.TTL == 0 {
		sp.Defaults.TTL = DefaultTTL
	}
	if sp.Defaults.NegativeTTL == 0 {
		sp.Defaults.NegativeTTL = DefaultNegativeTTL
	}
	if sp.Defaults.CNAMEDepth == 0 {
		sp.Defaults.CNAMEDepth = model.DefaultCNAMEDepth
	}
	if sp.Zones == nil {
		sp.Zones = []model.Zone{}
	}
	if sp.Forwarding.Policies == nil {
		sp.Forwarding.Policies = []model.ForwardingPolicy{}
	}
	if sp.Forwarding.Pools == nil {
		sp.Forwarding.Pools = []model.UpstreamPool{}
	}
	if sp.Chaos.Policies == nil {
		sp.Chaos.Policies = []model.ChaosPolicy{}
	}
	if sp.Management.Auth.Profile == "" {
		sp.Management.Auth.Profile = model.AuthProfileDevLoopbackUnauth
	}
	for zi := range sp.Zones {
		z := &sp.Zones[zi]
		if z.Records == nil {
			z.Records = []model.Record{}
		}
		if z.Nameservers == nil {
			z.Nameservers = []model.Name{}
		}
		for ri := range z.Records {
			r := &z.Records[ri]
			if r.TTL == 0 {
				r.TTL = sp.Defaults.TTL
			}
		}
	}
	for pi := range sp.Forwarding.Pools {
		p := &sp.Forwarding.Pools[pi]
		if p.Upstreams == nil {
			p.Upstreams = []model.Upstream{}
		}
	}
}

func canonicalizeNames(sp *model.Spec, vs *[]domainerr.FieldViolation) {
	for zi := range sp.Zones {
		z := &sp.Zones[zi]
		path := indexPath("spec.zones", zi)
		n, viol := CanonicalName(string(z.Name), "")
		if viol != nil {
			viol.Path = path + ".name"
			*vs = append(*vs, *viol)
		} else {
			z.Name = n
		}
		if z.SOA != nil {
			if p, viol := CanonicalName(string(z.SOA.Primary), z.Name); viol != nil {
				viol.Path = path + ".soa.primary"
				*vs = append(*vs, *viol)
			} else {
				z.SOA.Primary = p
			}
			if a, viol := CanonicalName(string(z.SOA.Administrator), z.Name); viol != nil {
				viol.Path = path + ".soa.administrator"
				*vs = append(*vs, *viol)
			} else {
				z.SOA.Administrator = a
			}
		}
		for ni := range z.Nameservers {
			if ns, viol := CanonicalName(string(z.Nameservers[ni]), z.Name); viol != nil {
				viol.Path = indexPath(path+".nameservers", ni)
				*vs = append(*vs, *viol)
			} else {
				z.Nameservers[ni] = ns
			}
		}
		for ri := range z.Records {
			r := &z.Records[ri]
			rp := indexPath(path+".records", ri)
			r.Type = canonicalizeRRType(r.Type)
			if owner, viol := CanonicalName(r.Owner, z.Name); viol != nil {
				viol.Path = rp + ".owner"
				*vs = append(*vs, *viol)
			} else {
				r.Owner = string(owner)
			}
			for vi := range r.Values {
				nv, viol := canonicalizeRecordValue(r.Type, r.Values[vi], z.Name)
				if viol != nil {
					viol.Path = indexPath(rp+".values", vi)
					*vs = append(*vs, *viol)
					continue
				}
				r.Values[vi] = nv
			}
		}
	}
	for pi := range sp.Forwarding.Policies {
		p := &sp.Forwarding.Policies[pi]
		path := indexPath("spec.forwarding.policies", pi)
		if string(p.Suffix) == "" {
			continue
		}
		if n, viol := CanonicalName(string(p.Suffix), ""); viol != nil {
			viol.Path = path + ".suffix"
			*vs = append(*vs, *viol)
		} else {
			p.Suffix = n
		}
	}
	for i := range sp.Chaos.Safety.ProtectedNames {
		if n, viol := CanonicalName(string(sp.Chaos.Safety.ProtectedNames[i]), ""); viol != nil {
			viol.Path = indexPath("spec.chaos.safety.protectedNames", i)
			*vs = append(*vs, *viol)
		} else {
			sp.Chaos.Safety.ProtectedNames[i] = n
		}
	}
	for pi := range sp.Chaos.Policies {
		p := &sp.Chaos.Policies[pi]
		path := indexPath("spec.chaos.policies", pi)
		for oi := range p.Scope.Owners {
			if n, viol := CanonicalName(string(p.Scope.Owners[oi]), ""); viol != nil {
				viol.Path = indexPath(path+".scope.owners", oi)
				*vs = append(*vs, *viol)
			} else {
				p.Scope.Owners[oi] = n
			}
		}
	}
}

func canonicalizeRRType(t model.RRType) model.RRType {
	if t == "" {
		return t
	}
	return model.RRType(strings.ToUpper(string(t)))
}

func canonicalizeRecordValue(t model.RRType, v string, origin model.Name) (string, *domainerr.FieldViolation) {
	switch t {
	case model.TypeCNAME, model.TypeNS, model.TypePTR:
		n, viol := CanonicalName(v, origin)
		if viol != nil {
			return "", viol
		}
		return string(n), nil
	case model.TypeMX:
		pref, name, ok := strings.Cut(strings.TrimSpace(v), " ")
		if !ok {
			return v, nil
		}
		n, viol := CanonicalName(name, origin)
		if viol != nil {
			return "", viol
		}
		return pref + " " + string(n), nil
	case model.TypeSRV:
		parts := strings.Fields(v)
		if len(parts) != 4 {
			return v, nil
		}
		n, viol := CanonicalName(parts[3], origin)
		if viol != nil {
			return "", viol
		}
		parts[3] = string(n)
		return strings.Join(parts, " "), nil
	case model.TypeSVCB, model.TypeHTTPS:
		parts := strings.Fields(v)
		if len(parts) < 2 || parts[1] == "." {
			return v, nil
		}
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return v, nil
		}
		n, viol := CanonicalName(parts[1], origin)
		if viol != nil {
			return "", viol
		}
		parts[1] = string(n)
		return strings.Join(parts, " "), nil
	default:
		return v, nil
	}
}
