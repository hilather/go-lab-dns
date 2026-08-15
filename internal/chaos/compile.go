package chaos

import (
	"fmt"
	"strconv"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// Compile builds an immutable ChaosIndex from canonical state.
//
// Zero-value / nil state yields a compiled-empty index. High-impact caps
// are enforced here so YAML cannot exceed Safety.MaxActiveHighImpactPolicies.
func Compile(st *model.State) (snapshot.ChaosIndex, error) {
	idx := snapshot.ChaosIndex{
		ByID:             map[model.PolicyID]*snapshot.CompiledChaos{},
		ByRecord:         map[model.RecordID][]model.PolicyID{},
		ByWildcardSource: map[model.RecordID][]model.PolicyID{},
		ByOwner:          map[model.Name][]model.PolicyID{},
		ByZone:           map[model.ZoneID][]model.PolicyID{},
		ByForwarding:     map[model.PolicyID][]model.PolicyID{},
		ByPool:           map[model.PoolID][]model.PolicyID{},
		ByClientGroup:    map[model.ClientGroupID][]model.PolicyID{},
		Global:           []model.PolicyID{},
	}
	if st == nil {
		return idx, nil
	}
	idx.Enabled = st.Spec.Chaos.Enabled

	high := 0
	for i := range st.Spec.Chaos.Policies {
		p := copyPolicy(st.Spec.Chaos.Policies[i])
		if p.ID == "" {
			return snapshot.ChaosIndex{}, domainerr.ValidationFailed("chaos policy missing id",
				domainerr.FieldViolation{Path: "spec.chaos.policies[" + strconv.Itoa(i) + "].id", Code: "empty_id", Message: "policy id is required"})
		}
		if _, dup := idx.ByID[p.ID]; dup {
			return snapshot.ChaosIndex{}, domainerr.ValidationFailed("duplicate chaos policy id",
				domainerr.FieldViolation{Path: "spec.chaos.policies[" + strconv.Itoa(i) + "].id", Code: "duplicate_id", Message: "duplicate policy id " + string(p.ID)})
		}
		if p.Enabled && p.SafetyClass == model.SafetyClassHigh {
			high++
		}
		prec := scopePrecedence(p.Scope)
		cp := &snapshot.CompiledChaos{Policy: p, Precedence: prec}
		idx.ByID[p.ID] = cp
		indexPolicy(&idx, p, prec)
	}

	// Record.chaosPolicyRefs attach the policy at record precedence.
	for zi := range st.Spec.Zones {
		z := st.Spec.Zones[zi]
		for ri := range z.Records {
			r := z.Records[ri]
			for _, ref := range r.ChaosPolicyRefs {
				cp, ok := idx.ByID[ref]
				if !ok {
					return snapshot.ChaosIndex{}, domainerr.ValidationFailed("unresolved chaosPolicyRef",
						domainerr.FieldViolation{
							Path:    fmt.Sprintf("spec.zones[%d].records[%d].chaosPolicyRefs", zi, ri),
							Code:    "unresolved_reference",
							Message: "chaos policy " + string(ref) + " does not exist",
						})
				}
				idx.ByRecord[r.ID] = appendUniqueRecord(idx.ByRecord[r.ID], ref)
				if cp.Precedence > snapshot.ChaosPrecRecord {
					cp.Precedence = snapshot.ChaosPrecRecord
				}
			}
		}
	}

	maxHigh := st.Spec.Chaos.Safety.MaxActiveHighImpactPolicies
	if maxHigh > 0 && high > maxHigh {
		return snapshot.ChaosIndex{}, domainerr.ValidationFailed("too many active high-impact chaos policies",
			domainerr.FieldViolation{
				Path:    "spec.chaos.policies",
				Code:    "invalid_value",
				Message: fmt.Sprintf("%d enabled high-impact policies exceed maxActiveHighImpactPolicies %d", high, maxHigh),
			})
	}
	return idx, nil
}

func indexPolicy(idx *snapshot.ChaosIndex, p model.ChaosPolicy, prec int) {
	s := p.Scope
	switch prec {
	case snapshot.ChaosPrecRecord:
		for _, id := range s.RecordIDs {
			idx.ByRecord[id] = appendUniqueRecord(idx.ByRecord[id], p.ID)
		}
	case snapshot.ChaosPrecWildcard:
		for _, id := range s.WildcardSourceIDs {
			idx.ByWildcardSource[id] = appendUniqueRecord(idx.ByWildcardSource[id], p.ID)
		}
	case snapshot.ChaosPrecOwner:
		for _, n := range s.Owners {
			idx.ByOwner[n] = appendUniquePolicy(idx.ByOwner[n], p.ID)
		}
	case snapshot.ChaosPrecZone:
		for _, id := range s.Zones {
			idx.ByZone[id] = appendUniquePolicy(idx.ByZone[id], p.ID)
		}
	case snapshot.ChaosPrecForward:
		for _, id := range s.ForwardingIDs {
			idx.ByForwarding[id] = appendUniquePolicy(idx.ByForwarding[id], p.ID)
		}
	case snapshot.ChaosPrecPool:
		for _, id := range s.UpstreamPools {
			idx.ByPool[id] = appendUniquePool(idx.ByPool[id], p.ID)
		}
	case snapshot.ChaosPrecGroup:
		for _, id := range s.ClientGroups {
			idx.ByClientGroup[id] = appendUniqueGroup(idx.ByClientGroup[id], p.ID)
		}
	default:
		idx.Global = appendUniquePolicy(idx.Global, p.ID)
	}
	// A record-precedence policy may also list wildcard sources; keep both
	// indexes so Candidates sees it from either contributing ID.
	if prec == snapshot.ChaosPrecRecord {
		for _, id := range s.WildcardSourceIDs {
			idx.ByWildcardSource[id] = appendUniqueRecord(idx.ByWildcardSource[id], p.ID)
		}
	}
}

func scopePrecedence(s model.ChaosScope) int {
	if len(s.RecordIDs) > 0 {
		return snapshot.ChaosPrecRecord
	}
	if len(s.WildcardSourceIDs) > 0 {
		return snapshot.ChaosPrecWildcard
	}
	if len(s.Owners) > 0 {
		return snapshot.ChaosPrecOwner
	}
	if len(s.Zones) > 0 {
		return snapshot.ChaosPrecZone
	}
	if len(s.ForwardingIDs) > 0 {
		return snapshot.ChaosPrecForward
	}
	if len(s.UpstreamPools) > 0 {
		return snapshot.ChaosPrecPool
	}
	if len(s.ClientGroups) > 0 {
		return snapshot.ChaosPrecGroup
	}
	return snapshot.ChaosPrecGlobal
}

func appendUniqueRecord(ids []model.PolicyID, id model.PolicyID) []model.PolicyID {
	return appendUniquePolicy(ids, id)
}

func appendUniquePool(ids []model.PolicyID, id model.PolicyID) []model.PolicyID {
	return appendUniquePolicy(ids, id)
}

func appendUniqueGroup(ids []model.PolicyID, id model.PolicyID) []model.PolicyID {
	return appendUniquePolicy(ids, id)
}

func appendUniquePolicy(ids []model.PolicyID, id model.PolicyID) []model.PolicyID {
	for _, e := range ids {
		if e == id {
			return ids
		}
	}
	return append(ids, id)
}

func copyPolicy(p model.ChaosPolicy) model.ChaosPolicy {
	if p.Labels != nil {
		labels := make(map[string]string, len(p.Labels))
		for k, v := range p.Labels {
			labels[k] = v
		}
		p.Labels = labels
	}
	if p.StartsAt != nil {
		t := *p.StartsAt
		p.StartsAt = &t
	}
	if p.ExpiresAt != nil {
		t := *p.ExpiresAt
		p.ExpiresAt = &t
	}
	p.Scope.RecordIDs = append([]model.RecordID(nil), p.Scope.RecordIDs...)
	p.Scope.Owners = append([]model.Name(nil), p.Scope.Owners...)
	p.Scope.WildcardSourceIDs = append([]model.RecordID(nil), p.Scope.WildcardSourceIDs...)
	p.Scope.Zones = append([]model.ZoneID(nil), p.Scope.Zones...)
	p.Scope.ForwardingIDs = append([]model.PolicyID(nil), p.Scope.ForwardingIDs...)
	p.Scope.UpstreamPools = append([]model.PoolID(nil), p.Scope.UpstreamPools...)
	p.Scope.ClientGroups = append([]model.ClientGroupID(nil), p.Scope.ClientGroups...)
	p.Scope.QTypes = append([]model.RRType(nil), p.Scope.QTypes...)
	p.Scope.Transports = append([]model.Transport(nil), p.Scope.Transports...)
	if p.Budget != nil {
		b := *p.Budget
		p.Budget = &b
	}
	if p.Outcomes != nil {
		outs := make([]model.ChaosOutcome, len(p.Outcomes))
		for i, o := range p.Outcomes {
			outs[i] = o
			if o.Actions != nil {
				acts := make([]model.ChaosAction, len(o.Actions))
				for j, a := range o.Actions {
					acts[j] = a
					acts[j].Values = append([]string(nil), a.Values...)
					if a.EDE != nil {
						e := *a.EDE
						acts[j].EDE = &e
					}
				}
				outs[i].Actions = acts
			}
		}
		p.Outcomes = outs
	}
	return p
}
