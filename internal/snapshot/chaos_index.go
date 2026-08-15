package snapshot

import "github.com/hilather/go-lab-dns/internal/model"

// Scope precedence (evaluation order). Lower is more specific.
const (
	ChaosPrecRecord   = 1
	ChaosPrecWildcard = 2
	ChaosPrecOwner    = 3
	ChaosPrecZone     = 4
	ChaosPrecForward  = 5
	ChaosPrecPool     = 6
	ChaosPrecGroup    = 7
	ChaosPrecGlobal   = 8
)

// ChaosIndex is the compiled chaos policy structure. Zero value is valid
// (no policies; Lookup and Matching miss).
//
// After chaos.Compile returns, the index is immutable. Callers must not
// mutate ByID or the scope slices.
type ChaosIndex struct {
	// Enabled is spec.chaos.enabled. Execution also consults EmergencyChaosOff.
	Enabled bool
	// ByID is every compiled policy. Non-nil after Compile, including empty.
	ByID map[model.PolicyID]*CompiledChaos
	// Scope indexes hold policy IDs in configuration order.
	ByRecord         map[model.RecordID][]model.PolicyID
	ByWildcardSource map[model.RecordID][]model.PolicyID
	ByOwner          map[model.Name][]model.PolicyID
	ByZone           map[model.ZoneID][]model.PolicyID
	ByForwarding     map[model.PolicyID][]model.PolicyID
	ByPool           map[model.PoolID][]model.PolicyID
	ByClientGroup    map[model.ClientGroupID][]model.PolicyID
	Global           []model.PolicyID
}

// CompiledChaos is one immutable compiled policy plus its precedence class.
type CompiledChaos struct {
	Policy     model.ChaosPolicy
	Precedence int
}

// Compiled reports whether chaos.Compile produced this index.
func (c ChaosIndex) Compiled() bool {
	return c.ByID != nil
}

// Lookup returns the compiled policy for id.
func (c ChaosIndex) Lookup(id model.PolicyID) (*CompiledChaos, bool) {
	if id == "" || c.ByID == nil {
		return nil, false
	}
	p, ok := c.ByID[id]
	return p, ok && p != nil
}

// ChaosMatch is the already-classified lookup key. Engine must not
// rediscover zone or forwarding from QNAME.
type ChaosMatch struct {
	RecordIDs      []model.RecordID
	WildcardSource model.RecordID
	Owner          model.Name
	ZoneID         model.ZoneID
	ForwardingID   model.PolicyID
	PoolID         model.PoolID
	ClientGroup    model.ClientGroupID
}

// Candidates returns matching compiled policies in precedence order
// (record → wildcard → owner → zone → forwarding → pool → group → global).
// Within a class, configuration order is preserved. A policy is emitted
// at most once, at its most specific class.
func (c ChaosIndex) Candidates(m ChaosMatch) []*CompiledChaos {
	if c.ByID == nil {
		return nil
	}
	seen := make(map[model.PolicyID]struct{}, len(c.ByID))
	out := make([]*CompiledChaos, 0, 8)
	add := func(ids []model.PolicyID) {
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			p := c.ByID[id]
			if p == nil {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, p)
		}
	}
	for _, rid := range m.RecordIDs {
		add(c.ByRecord[rid])
	}
	if m.WildcardSource != "" {
		add(c.ByWildcardSource[m.WildcardSource])
	}
	if m.Owner != "" {
		add(c.ByOwner[m.Owner])
	}
	if m.ZoneID != "" {
		add(c.ByZone[m.ZoneID])
	}
	if m.ForwardingID != "" {
		add(c.ByForwarding[m.ForwardingID])
	}
	if m.PoolID != "" {
		add(c.ByPool[m.PoolID])
	}
	if m.ClientGroup != "" {
		add(c.ByClientGroup[m.ClientGroup])
	}
	add(c.Global)
	return out
}
