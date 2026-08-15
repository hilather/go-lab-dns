package model

import "time"

// UnknownClientRefuseForward is the only v1alpha1 value of AccessSpec.UnknownClient.
// It refuses recursion/forwarding for unmatched clients; local answers still serve.
const UnknownClientRefuseForward = "refuse-forward"

// DefaultCNAMEDepth is the safe default CNAME chain cap, including overlay
// CNAME chains that terminate in a forwarded name.
const DefaultCNAMEDepth = 8

const (
	AuthProfileDevLoopbackUnauth = "dev-loopback-unauth"
	AuthProfileBearer            = "bearer"
)

// ListenersSpec configures the DNS and management listeners.
type ListenersSpec struct {
	DNS        DNSListenerSpec  `json:"dns"`
	Management MgmtListenerSpec `json:"management"`
}

// DNSListenerSpec is the data-plane listener.
type DNSListenerSpec struct {
	Address   string      `json:"address"`
	Protocols []Transport `json:"protocols"`
}

// MgmtListenerSpec is the control-plane HTTP listener.
type MgmtListenerSpec struct {
	Address  string `json:"address"`
	RESTPath string `json:"restPath"`
	MCPPath  string `json:"mcpPath"`
}

// AccessSpec classifies clients and gates forwarding.
type AccessSpec struct {
	UnknownClient string        `json:"unknownClient"`
	ClientGroups  []ClientGroup `json:"clientGroups"`
}

// ClientGroup is a CIDR-keyed client class.
type ClientGroup struct {
	ID           ClientGroupID `json:"id"`
	CIDRs        []string      `json:"cidrs"`
	ChaosExempt  bool          `json:"chaosExempt"`
	AllowForward bool          `json:"allowForward"`
}

// DefaultsSpec holds materialized query defaults.
type DefaultsSpec struct {
	TTL         time.Duration `json:"ttl"`
	NegativeTTL time.Duration `json:"negativeTTL"`
	CNAMEDepth  int           `json:"cnameDepth"`
}

// CacheSpec is cache bounds only; entries live outside Snapshot.
type CacheSpec struct {
	Enabled            bool          `json:"enabled"`
	MaxEntries         int           `json:"maxEntries"`
	MinimumTTL         time.Duration `json:"minimumTTL"`
	MaximumTTL         time.Duration `json:"maximumTTL"`
	MaximumNegativeTTL time.Duration `json:"maximumNegativeTTL"`
	StaleServing       bool          `json:"staleServing"`
}

// ObservabilitySpec is process telemetry configuration.
type ObservabilitySpec struct {
	LogQNAME bool `json:"logQNAME"`
}

// ManagementSpec is control-plane authentication configuration.
type ManagementSpec struct {
	Auth AuthSpec `json:"auth"`
}

// AuthSpec names an auth profile and optional secret references.
type AuthSpec struct {
	Profile   string `json:"profile"`
	SecretRef string `json:"secretRef,omitempty"`
}
