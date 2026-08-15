package capabilities

// Frozen first-GA scopes. Role binding lands in SEC-001; adapters must not
// invent synonyms.
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
