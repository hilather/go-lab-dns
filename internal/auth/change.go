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
	case model.TargetChaosSafety, model.TargetManagement, model.TargetAccess,
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
	enabled := false
	class := model.SafetyClassLow
	if hasVal {
		enabled = pol.Enabled
		class = pol.SafetyClass
	}
	if prev != nil && !hasVal {
		class = prev.SafetyClass
	}
	if op.Op == model.OpRemove {
		if !actor.HasScope(ScopeChaosWrite) {
			return domainerr.Forbidden("missing scope " + ScopeChaosWrite)
		}
		if prev != nil && prev.Enabled && !actor.HasScope(ScopeChaosActivate) {
			return domainerr.Forbidden("removing an active policy requires " + ScopeChaosActivate)
		}
		return nil
	}
	if enabled {
		if err := authorizeActivation(actor, class); err != nil {
			return err
		}
		// Creating an enabled policy also needs design privilege unless this
		// is a pure activation of an existing object (use TargetChaosActivation).
		if prev == nil && !actor.HasScope(ScopeChaosWrite) {
			return domainerr.Forbidden("missing scope " + ScopeChaosWrite)
		}
		return nil
	}
	if !actor.HasScope(ScopeChaosWrite) {
		return domainerr.Forbidden("missing scope " + ScopeChaosWrite)
	}
	return nil
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
	case model.TargetForwardingPolicy, model.TargetUpstreamPool, model.TargetUpstream:
		return []string{ScopeForwardersWrite}
	case model.TargetChaosPolicy:
		pol, has, _ := decodeChaosPolicy(op)
		if has && pol.Enabled {
			if pol.SafetyClass == model.SafetyClassHigh {
				return []string{ScopeChaosWrite, ScopeChaosActivate, ScopeChaosEmergency}
			}
			return []string{ScopeChaosWrite, ScopeChaosActivate}
		}
		return []string{ScopeChaosWrite}
	case model.TargetChaosActivation:
		prev := findChaos(current, model.PolicyID(op.Target.ID))
		if prev != nil && prev.SafetyClass == model.SafetyClassHigh {
			return []string{ScopeChaosActivate, ScopeChaosEmergency}
		}
		return []string{ScopeChaosActivate}
	case model.TargetChaosSafety, model.TargetManagement, model.TargetAccess,
		model.TargetListeners, model.TargetCache, model.TargetDefaults,
		model.TargetObservability, model.TargetClientGroup:
		return []string{ScopeDNSAdmin}
	default:
		return []string{ScopeDNSWrite}
	}
}
