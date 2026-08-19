package auth

// Frozen first-GA scopes. Spellings match internal/capabilities; adapters
// must not invent synonyms.
const (
	ScopeDNSRead         = "dns.read"
	ScopeDNSWrite        = "dns.write"
	ScopeDNSAdmin        = "dns.admin"
	ScopeForwardersRead  = "dns.forwarders.read"
	ScopeForwardersWrite = "dns.forwarders.write"
	ScopeChaosRead       = "dns.chaos.read"
	ScopeChaosWrite      = "dns.chaos.write"
	ScopeChaosActivate   = "dns.chaos.activate"
	ScopeChaosEmergency  = "dns.chaos.emergency"
	ScopeAuditRead       = "dns.audit.read"
)

// AllScopes is the closed first-GA catalog in documented order.
// Scope spellings must remain identical to internal/capabilities.

func AllScopes() []string {
	return []string{
		ScopeDNSRead,
		ScopeDNSWrite,
		ScopeDNSAdmin,
		ScopeForwardersRead,
		ScopeForwardersWrite,
		ScopeChaosRead,
		ScopeChaosWrite,
		ScopeChaosActivate,
		ScopeChaosEmergency,
		ScopeAuditRead,
	}
}

// Resource-aware write capabilities: catalog RequiredScopes is dns.write,
// but the change set decides the actual scopes.
const (
	CapStateValidate = "state.validate"
	CapChangePlan    = "change.plan"
	CapChangeApply   = "change.apply"
)

// EffectiveScopes is the scope set HasScope consults (explicit scopes, role expansion, or class).
func (a Actor) EffectiveScopes() []string {
	return a.effectiveScopes()
}

func (a Actor) effectiveScopes() []string {
	if len(a.Scopes) > 0 {
		if a.Role == RoleAdministrator || contains(a.Scopes, ScopeDNSAdmin) {
			return AllScopes()
		}
		return append([]string(nil), a.Scopes...)
	}
	if a.Role != "" {
		return RoleScopes(a.Role)
	}
	switch a.Class {
	case ClassLoopback, ClassStartup, ClassLocalSignal:
		return AllScopes()
	default:
		return nil
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// HasScope reports whether a holds scope. dns.admin satisfies every scope.
func (a Actor) HasScope(scope string) bool {
	if scope == "" {
		return true
	}
	for _, s := range a.effectiveScopes() {
		if s == scope || s == ScopeDNSAdmin {
			return true
		}
	}
	return false
}

// CanMutate is true when the actor may call plan/apply/validate at all.
// Fine-grained target checks happen in AuthorizeChange.
func (a Actor) CanMutate() bool {
	return a.HasScope(ScopeDNSWrite) ||
		a.HasScope(ScopeForwardersWrite) ||
		a.HasScope(ScopeChaosWrite) ||
		a.HasScope(ScopeChaosActivate) ||
		a.HasScope(ScopeChaosEmergency) ||
		a.HasScope(ScopeDNSAdmin)
}

// CanActivateHigh is chaos-admin or administrator (activate + emergency/admin).
func (a Actor) CanActivateHigh() bool {
	return a.HasScope(ScopeChaosActivate) && (a.HasScope(ScopeChaosEmergency) || a.HasScope(ScopeDNSAdmin))
}

// CanEmergencyEnable is chaos-admin or administrator. Emergency-only
// operators may disable but not re-enable.
func (a Actor) CanEmergencyEnable() bool {
	return a.HasScope(ScopeChaosEmergency) && (a.HasScope(ScopeChaosActivate) || a.HasScope(ScopeDNSAdmin))
}
