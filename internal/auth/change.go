package auth

import (
	"bytes"
	"encoding/json"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

// AuthorizeChange is the resource-aware check for plan/apply/validate.
func AuthorizeChange(actor Actor, ops []model.Operation, current *model.State) error {
	if current == nil {
		current = &model.State{}
	}
	if len(ops) == 0 {
		if !actor.HasScope(ScopeDNSWrite) && !actor.HasScope(ScopeDNSAdmin) {
			return domainerr.Forbidden("empty change requires dns.write")
		}
		return nil
	}
	prot := ProtectFrom(current)
	for _, op := range ops {
		if err := authorizeOp(actor, op, current, prot); err != nil {
			return err
		}
	}
	return nil
}

func authorizeOp(actor Actor, op model.Operation, current *model.State, prot Protected) error {
	admin := actor.HasScope(ScopeDNSAdmin)
	switch op.Target.Kind {
	case model.TargetZone:
		if !actor.HasScope(ScopeDNSWrite) {
			return domainerr.Forbidden("missing scope " + ScopeDNSWrite)
		}
		if !admin && (prot.hasZone(model.ZoneID(op.Target.ID)) || zoneTouchesProtected(op, prot)) {
			return denyProtected("protected zone " + op.Target.ID)
		}
		return nil
	case model.TargetRecord:
		if !actor.HasScope(ScopeDNSWrite) {
			return domainerr.Forbidden("missing scope " + ScopeDNSWrite)
		}
		if !admin && (prot.hasRecord(model.RecordID(op.Target.ID)) || recordTouchesProtected(op, prot)) {
			return denyProtected("protected record " + op.Target.ID)
		}
		if !admin && prot.hasZone(op.Target.ZoneID) {
			return denyProtected("protected zone " + string(op.Target.ZoneID))
		}
		return nil
	case model.TargetForwardingPolicy:
		if !actor.HasScope(ScopeForwardersWrite) {
			return domainerr.Forbidden("missing scope " + ScopeForwardersWrite)
		}
		return nil
	case model.TargetUpstreamPool:
		if !actor.HasScope(ScopeForwardersWrite) {
			return domainerr.Forbidden("missing scope " + ScopeForwardersWrite)
		}
		if !admin && poolEndpointChanges(op, current) {
			return denyProtected("upstream endpoints require administrator")
		}
		return nil
	case model.TargetUpstream:
		if !actor.HasScope(ScopeForwardersWrite) {
			return domainerr.Forbidden("missing scope " + ScopeForwardersWrite)
		}
		if !admin && upstreamEndpointChanges(op, current) {
			return denyProtected("upstream endpoints require administrator")
		}
		return nil
	case model.TargetClientGroup:
		if !admin {
			if prot.hasGroup(model.ClientGroupID(op.Target.ID)) {
				return denyProtected("protected client group " + op.Target.ID)
			}
			return domainerr.Forbidden("client groups require " + ScopeDNSAdmin)
		}
		return nil
	case model.TargetChaosPolicy:
		return authorizeChaosPolicy(actor, op, current)
	case model.TargetChaosActivation:
		return authorizeChaosActivation(actor, op, current)
	case model.TargetChaosSafety, model.TargetManagement, model.TargetUI, model.TargetAccess,
		model.TargetListeners, model.TargetCache, model.TargetDefaults, model.TargetObservability:
		if !admin {
			return domainerr.Forbidden("target " + string(op.Target.Kind) + " requires " + ScopeDNSAdmin)
		}
		return nil
	default:
		return domainerr.ValidationFailed("unknown target kind",
			domainerr.FieldViolation{Path: "target.kind", Code: "invalid_value", Message: "unknown target kind"})
	}
}

func authorizeChaosPolicy(actor Actor, op model.Operation, current *model.State) error {
	pol, hasVal, err := decodeChaosPolicy(op)
	if err != nil {
		return err
	}
	prev := findChaos(current, model.PolicyID(op.Target.ID))
	// Design path always: operators activate via TargetChaosActivation only.
	if !actor.HasScope(ScopeChaosWrite) {
		return domainerr.Forbidden("missing scope " + ScopeChaosWrite)
	}
	if op.Op == model.OpRemove {
		if prev != nil && prev.Enabled && !actor.HasScope(ScopeChaosActivate) {
			return domainerr.Forbidden("removing an active policy requires " + ScopeChaosActivate)
		}
		return nil
	}
	enabled := hasVal && pol.Enabled
	// Mutating or disabling a live policy is an activation privilege.
	if prev != nil && prev.Enabled && !actor.HasScope(ScopeChaosActivate) {
		return domainerr.Forbidden("changing an active policy requires " + ScopeChaosActivate)
	}
	if !enabled {
		return nil
	}
	class := pol.SafetyClass
	if prev != nil {
		class = stricterClass(prev.SafetyClass, class)
	}
	return authorizeActivation(actor, class)
}

func stricterClass(a, b model.SafetyClass) model.SafetyClass {
	if classRank(b) > classRank(a) {
		return b
	}
	return a
}

func classRank(c model.SafetyClass) int {
	switch c {
	case model.SafetyClassHigh, model.SafetyClassUnsafeDeferred:
		return 2
	case model.SafetyClassMedium:
		return 1
	default:
		return 0
	}
}

func authorizeChaosActivation(actor Actor, op model.Operation, current *model.State) error {
	var act model.ChaosActivation
	if len(bytes.TrimSpace(op.Value)) > 0 {
		if err := json.Unmarshal(op.Value, &act); err != nil {
			return domainerr.ValidationFailed("invalid chaos activation",
				domainerr.FieldViolation{Path: "value", Code: "invalid_value", Message: "value is not a chaos activation"})
		}
	}
	prev := findChaos(current, model.PolicyID(op.Target.ID))
	class := model.SafetyClassLow
	if prev != nil {
		class = prev.SafetyClass
	}
	if act.Enabled {
		return authorizeActivation(actor, class)
	}
	if !actor.HasScope(ScopeChaosActivate) {
		return domainerr.Forbidden("missing scope " + ScopeChaosActivate)
	}
	return nil
}

func authorizeActivation(actor Actor, class model.SafetyClass) error {
	if !actor.HasScope(ScopeChaosActivate) {
		return domainerr.Forbidden("missing scope " + ScopeChaosActivate)
	}
	if class == model.SafetyClassHigh && !actor.CanActivateHigh() {
		return domainerr.Forbidden("high-impact activation requires chaos-admin")
	}
	return nil
}

func decodeChaosPolicy(op model.Operation) (model.ChaosPolicy, bool, error) {
	if len(bytes.TrimSpace(op.Value)) == 0 {
		return model.ChaosPolicy{}, false, nil
	}
	var p model.ChaosPolicy
	if err := json.Unmarshal(op.Value, &p); err != nil {
		return model.ChaosPolicy{}, false, domainerr.ValidationFailed("invalid chaos policy",
			domainerr.FieldViolation{Path: "value", Code: "invalid_value", Message: "value is not a chaos policy"})
	}
	return p, true, nil
}

func findChaos(st *model.State, id model.PolicyID) *model.ChaosPolicy {
	if st == nil || id == "" {
		return nil
	}
	for i := range st.Spec.Chaos.Policies {
		if st.Spec.Chaos.Policies[i].ID == id {
			p := st.Spec.Chaos.Policies[i]
			return &p
		}
	}
	return nil
}

func findPool(st *model.State, id model.PoolID) *model.UpstreamPool {
	if st == nil || id == "" {
		return nil
	}
	for i := range st.Spec.Forwarding.Pools {
		if st.Spec.Forwarding.Pools[i].ID == id {
			p := st.Spec.Forwarding.Pools[i]
			return &p
		}
	}
	return nil
}

func findUpstream(st *model.State, id model.UpstreamID) *model.Upstream {
	if st == nil || id == "" {
		return nil
	}
	for _, pool := range st.Spec.Forwarding.Pools {
		for i := range pool.Upstreams {
			if pool.Upstreams[i].ID == id {
				u := pool.Upstreams[i]
				return &u
			}
		}
	}
	return nil
}

func upstreamEndpointChanges(op model.Operation, current *model.State) bool {
	prev := findUpstream(current, model.UpstreamID(op.Target.ID))
	if prev == nil {
		return false
	}
	if op.Op == model.OpRemove {
		return true
	}
	if len(bytes.TrimSpace(op.Value)) == 0 {
		return false
	}
	var next model.Upstream
	if err := json.Unmarshal(op.Value, &next); err != nil {
		return true
	}
	return next.Endpoint != "" && next.Endpoint != prev.Endpoint
}

func poolEndpointChanges(op model.Operation, current *model.State) bool {
	prev := findPool(current, model.PoolID(op.Target.ID))
	if prev == nil {
		return false
	}
	if op.Op == model.OpRemove {
		return len(prev.Upstreams) > 0
	}
	if len(bytes.TrimSpace(op.Value)) == 0 {
		return false
	}
	var next model.UpstreamPool
	if err := json.Unmarshal(op.Value, &next); err != nil {
		return true
	}
	nextByID := make(map[model.UpstreamID]model.Upstream, len(next.Upstreams))
	for _, u := range next.Upstreams {
		nextByID[u.ID] = u
	}
	for _, u := range prev.Upstreams {
		n, ok := nextByID[u.ID]
		if !ok || n.Endpoint != u.Endpoint {
			return true
		}
	}
	return false
}

func zoneTouchesProtected(op model.Operation, prot Protected) bool {
	if len(bytes.TrimSpace(op.Value)) == 0 {
		return false
	}
	var z model.Zone
	if err := json.Unmarshal(op.Value, &z); err != nil {
		return false
	}
	if prot.coversName(string(z.Name)) {
		return true
	}
	for _, r := range z.Records {
		if prot.hasRecord(r.ID) || prot.coversName(r.Owner) {
			return true
		}
	}
	return false
}

func recordTouchesProtected(op model.Operation, prot Protected) bool {
	if len(bytes.TrimSpace(op.Value)) == 0 {
		return false
	}
	var r model.Record
	if err := json.Unmarshal(op.Value, &r); err != nil {
		return false
	}
	return prot.hasRecord(r.ID) || prot.coversName(r.Owner)
}

// RequiredPermissions lists the scopes a change set needs.
func RequiredPermissions(ops []model.Operation, current *model.State) []string {
	if len(ops) == 0 {
		return []string{ScopeDNSWrite}
	}
	var out []string
	for _, op := range ops {
		out = append(out, scopesForOp(op, current)...)
	}
	return uniqSorted(out)
}

func scopesForOp(op model.Operation, current *model.State) []string {
	switch op.Target.Kind {
	case model.TargetZone, model.TargetRecord:
		return []string{ScopeDNSWrite}
	case model.TargetForwardingPolicy:
		return []string{ScopeForwardersWrite}
	case model.TargetUpstreamPool:
		if poolEndpointChanges(op, current) {
			return []string{ScopeForwardersWrite, ScopeDNSAdmin}
		}
		return []string{ScopeForwardersWrite}
	case model.TargetUpstream:
		if upstreamEndpointChanges(op, current) {
			return []string{ScopeForwardersWrite, ScopeDNSAdmin}
		}
		return []string{ScopeForwardersWrite}
	case model.TargetChaosPolicy:
		need := []string{ScopeChaosWrite}
		pol, has, _ := decodeChaosPolicy(op)
		prev := findChaos(current, model.PolicyID(op.Target.ID))
		enabled := has && pol.Enabled
		if (prev != nil && prev.Enabled) || enabled {
			need = append(need, ScopeChaosActivate)
		}
		class := model.SafetyClassLow
		if has {
			class = pol.SafetyClass
		}
		if prev != nil {
			class = stricterClass(prev.SafetyClass, class)
		}
		if enabled && class == model.SafetyClassHigh {
			need = append(need, ScopeChaosEmergency)
		}
		return need
	case model.TargetChaosActivation:
		prev := findChaos(current, model.PolicyID(op.Target.ID))
		if prev != nil && prev.SafetyClass == model.SafetyClassHigh {
			return []string{ScopeChaosActivate, ScopeChaosEmergency}
		}
		return []string{ScopeChaosActivate}
	case model.TargetChaosSafety, model.TargetManagement, model.TargetUI, model.TargetAccess,
		model.TargetListeners, model.TargetCache, model.TargetDefaults,
		model.TargetObservability, model.TargetClientGroup:
		return []string{ScopeDNSAdmin}
	default:
		return []string{ScopeDNSWrite}
	}
}
