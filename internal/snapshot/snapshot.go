package snapshot

import (
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// Snapshot is immutable after Compile returns. The query cache is not a
// field; it is a process-scoped store namespaced by Revision.
type Snapshot struct {
	Canonical         *model.State
	Revision          model.Revision
	BootstrapRevision model.Revision
	Generation        model.Generation
	CompiledAt        time.Time

	Listeners         ListenerView
	Access            AccessIndex
	Defaults          DefaultsView
	Zones             ZoneIndex
	Forwarding        ForwardingIndex
	CachePolicy       CachePolicy
	Chaos             ChaosIndex
	Safety            SafetyPolicy
	Management        ManagementView
	Observability     ObservabilityView
	EmergencyChaosOff bool
}

// ZoneIndex is the compiled zone lookup structure. Zero value is valid.
type ZoneIndex struct{}

// ForwardingIndex is the compiled suffix-forwarding structure. Zero value is valid.
type ForwardingIndex struct{}

// ChaosIndex is the compiled chaos policy structure. Zero value is valid.
type ChaosIndex struct{}

// AccessIndex is the compiled CIDR-to-client-group structure. Zero value is valid.
type AccessIndex struct{}

// ListenerView is the compiled listener bind configuration.
type ListenerView struct {
	DNSAddress        string
	DNSProtocols      []model.Transport
	ManagementAddress string
	RESTPath          string
	MCPPath           string
}

// DefaultsView is the compiled query defaults.
type DefaultsView struct {
	TTL         time.Duration
	NegativeTTL time.Duration
	CNAMEDepth  int
}

// CachePolicy is cache bounds only; it does not hold entries.
type CachePolicy struct {
	Enabled            bool
	MaxEntries         int
	MinimumTTL         time.Duration
	MaximumTTL         time.Duration
	MaximumNegativeTTL time.Duration
	StaleServing       bool
}

// SafetyPolicy is compiled global chaos caps.
type SafetyPolicy struct {
	ProtectedNames                []model.Name
	ProtectedClientGroups         []model.ClientGroupID
	AllowedAddressCIDRs           []string
	MaxDelay                      time.Duration
	MaxConcurrentDelayed          int
	MaxDropProbability            float64
	MaxActiveHighImpactPolicies   int
	RequireExpiryForSafetyClasses []model.SafetyClass
	DefaultMaxLifetime            time.Duration
}

// ManagementView is compiled management-plane settings.
type ManagementView struct {
	AuthProfile string
}

// ObservabilityView is compiled telemetry settings.
type ObservabilityView struct {
	LogQNAME bool
}
