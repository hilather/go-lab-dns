package auth

import "sort"

// Suggested first-GA roles from docs/08-security-architecture.md.
const (
	RoleViewer            = "viewer"
	RoleDNSEditor         = "dns-editor"
	RoleForwarderOperator = "forwarder-operator"
	RoleChaosDesigner     = "chaos-designer"
	RoleChaosOperator     = "chaos-operator"
	RoleChaosAdmin        = "chaos-admin"
	RoleEmergencyOperator = "emergency-operator"
	RoleAdministrator     = "administrator"
)

// AllRoles is the closed first-GA role list in documented order.
func AllRoles() []string {
	return []string{
		RoleViewer,
		RoleDNSEditor,
		RoleForwarderOperator,
		RoleChaosDesigner,
		RoleChaosOperator,
		RoleChaosAdmin,
		RoleEmergencyOperator,
		RoleAdministrator,
	}
}

// RoleScopes is the frozen role→scope binding.
func RoleScopes(role string) []string {
	switch role {
	case RoleViewer:
		return []string{ScopeDNSRead, ScopeForwardersRead, ScopeChaosRead}
	case RoleDNSEditor:
		return []string{ScopeDNSRead, ScopeDNSWrite}
	case RoleForwarderOperator:
		return []string{ScopeDNSRead, ScopeForwardersRead, ScopeForwardersWrite}
	case RoleChaosDesigner:
		return []string{ScopeDNSRead, ScopeChaosRead, ScopeChaosWrite}
	case RoleChaosOperator:
		return []string{ScopeDNSRead, ScopeChaosRead, ScopeChaosActivate}
	case RoleChaosAdmin:
		return []string{
			ScopeDNSRead, ScopeChaosRead, ScopeChaosWrite,
			ScopeChaosActivate, ScopeChaosEmergency,
		}
	case RoleEmergencyOperator:
		return []string{ScopeDNSRead, ScopeChaosRead, ScopeChaosEmergency}
	case RoleAdministrator:
		return AllScopes()
	default:
		return nil
	}
}

// KnownRole reports whether role is one of the frozen first-GA roles.
func KnownRole(role string) bool {
	for _, r := range AllRoles() {
		if r == role {
			return true
		}
	}
	return false
}

func uniqSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
