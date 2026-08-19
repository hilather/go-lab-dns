package model

const (
	// APIVersionV1Alpha1 is the only first-GA config API version.
	APIVersionV1Alpha1 = "labdns.dev/v1alpha1"
	// KindLabDNS is the config document kind.
	KindLabDNS = "LabDNS"
)

// State is the canonical desired-state document.
type State struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}

// Metadata is document identity and labels.
type Metadata struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

// Spec is the v1alpha1 desired-state contract. YAML decode and default
// materialization live in config, not here.
type Spec struct {
	Listeners     ListenersSpec     `json:"listeners"`
	Access        AccessSpec        `json:"access"`
	Defaults      DefaultsSpec      `json:"defaults"`
	Zones         []Zone            `json:"zones"`
	Forwarding    ForwardingSpec    `json:"forwarding"`
	Cache         CacheSpec         `json:"cache"`
	Chaos         ChaosSpec         `json:"chaos"`
	Observability ObservabilitySpec `json:"observability"`
	UI            UISpec            `json:"ui"`
	Management    ManagementSpec    `json:"management"`
}

// DefaultUIEnabled is the materialized default for UISpec.Enabled when
// spec.ui or enabled is omitted. The Go zero value remains false until
// config materializes the field.
const DefaultUIEnabled = true

// UISpec toggles the embedded operator console. Serving vs 404 of SPA
// assets is read from the active snapshot; REST and MCP stay up.
type UISpec struct {
	Enabled bool `json:"enabled"`
}
