package auth

import "github.com/hilather/go-lab-dns/internal/domainerr"

// AuthorizeCapability checks catalog RequiredScopes. Plan/apply/validate
// are resource-aware: any mutate-family scope is enough to reach the
// change-set check.
func AuthorizeCapability(actor Actor, required []string, capID string) error {
	switch capID {
	case CapStateValidate, CapChangePlan, CapChangeApply:
		if actor.CanMutate() {
			return nil
		}
		return domainerr.Forbidden("insufficient scope for " + capID)
	}
	for _, s := range required {
		if !actor.HasScope(s) {
			return domainerr.Forbidden("missing scope " + s)
		}
	}
	return nil
}

// AuthorizeEmergency checks disable vs enable. Disable is emergency-only;
// enable requires chaos-admin (emergency + activate) or administrator.
func AuthorizeEmergency(actor Actor, enable bool) error {
	if !actor.HasScope(ScopeChaosEmergency) {
		return domainerr.Forbidden("missing scope " + ScopeChaosEmergency)
	}
	if enable && !actor.CanEmergencyEnable() {
		return domainerr.Forbidden("emergency enable requires chaos-admin or administrator")
	}
	return nil
}
