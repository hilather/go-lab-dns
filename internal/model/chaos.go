package model

import "time"

// SafetyClass is a chaos policy impact class.
type SafetyClass string

const (
	SafetyClassLow            SafetyClass = "low"
	SafetyClassMedium         SafetyClass = "medium"
	SafetyClassHigh           SafetyClass = "high"
	SafetyClassUnsafeDeferred SafetyClass = "unsafe-deferred"
)

// Composition controls whether lower-precedence policies still run.
type Composition string

const (
	CompositionCompose        Composition = "compose"
	CompositionTerminal       Composition = "terminal"
	CompositionExclusiveGroup Composition = "exclusive-group"
)

// SelectorMode is how a policy picks an outcome.
type SelectorMode string

const (
	SelectorDeterministic SelectorMode = "deterministic"
	SelectorRandom        SelectorMode = "random"
)

const (
	DistFixed   = "fixed"
	DistUniform = "uniform"
)

const (
	ActionDelay     = "delay"
	ActionRCode     = "rcode"
	ActionDrop      = "drop"
	ActionTruncate  = "truncate"
	ActionTCPClose  = "tcp-close"
	ActionTCPReset  = "tcp-reset"
	ActionTTL       = "ttl"
	ActionAlternate = "alternate"
	ActionOmit      = "omit"
	ActionLimit     = "limit"
	ActionShuffle   = "shuffle"
	ActionRotate    = "rotate"
	ActionCache     = "cache"
	ActionUpstream  = "upstream"
	ActionPressure  = "pressure"
)

const (
	PhaseBeforeResolution = "before-resolution"
	PhaseBeforeUpstream   = "before-upstream"
	PhaseAfterUpstream    = "after-upstream"
	PhaseBeforeResponse   = "before-response"
)

// ChaosSpec is the chaos engine configuration. Chaos is disabled by default.
type ChaosSpec struct {
	Enabled           bool          `json:"enabled"`
	EmergencyDisabled bool          `json:"emergencyDisabled"`
	Safety            SafetySpec    `json:"safety"`
	Policies          []ChaosPolicy `json:"policies"`
}

// SafetySpec is global chaos caps. Runtime flags may impose stricter caps.
type SafetySpec struct {
	ProtectedNames                []Name          `json:"protectedNames,omitempty"`
	ProtectedClientGroups         []ClientGroupID `json:"protectedClientGroups,omitempty"`
	AllowedAddressCIDRs           []string        `json:"allowedAddressCIDRs,omitempty"`
	MaxDelay                      time.Duration   `json:"maxDelay,omitempty"`
	MaxConcurrentDelayed          int             `json:"maxConcurrentDelayed,omitempty"`
	MaxDropProbability            float64         `json:"maxDropProbability,omitempty"`
	MaxActiveHighImpactPolicies   int             `json:"maxActiveHighImpactPolicies,omitempty"`
	RequireExpiryForSafetyClasses []SafetyClass   `json:"requireExpiryForSafetyClasses,omitempty"`
	DefaultMaxLifetime            time.Duration   `json:"defaultMaxLifetime,omitempty"`
}

// ChaosPolicy is one scoped, explainable fault policy.
type ChaosPolicy struct {
	ID             PolicyID          `json:"id"`
	Description    string            `json:"description,omitempty"`
	Owner          string            `json:"owner"`
	Labels         map[string]string `json:"labels,omitempty"`
	Reason         string            `json:"reason"`
	Ticket         string            `json:"ticket,omitempty"`
	Enabled        bool              `json:"enabled"`
	StartsAt       *time.Time        `json:"startsAt,omitempty"`
	ExpiresAt      *time.Time        `json:"expiresAt,omitempty"`
	SafetyClass    SafetyClass       `json:"safetyClass"`
	Scope          ChaosScope        `json:"scope"`
	Selector       ChaosSelector     `json:"selector"`
	Outcomes       []ChaosOutcome    `json:"outcomes"`
	Composition    Composition       `json:"composition,omitempty"`
	ExclusiveGroup string            `json:"exclusiveGroup,omitempty"`
	Budget         *ChaosBudget      `json:"budget,omitempty"`
}

// ChaosScope selects when a policy is eligible. Empty fields do not constrain.
type ChaosScope struct {
	RecordIDs         []RecordID      `json:"recordIds,omitempty"`
	Owners            []Name          `json:"owners,omitempty"`
	WildcardSourceIDs []RecordID      `json:"wildcardSourceIds,omitempty"`
	Zones             []ZoneID        `json:"zones,omitempty"`
	ForwardingIDs     []PolicyID      `json:"forwardingPolicyIds,omitempty"`
	UpstreamPools     []PoolID        `json:"upstreamPools,omitempty"`
	ClientGroups      []ClientGroupID `json:"clientGroups,omitempty"`
	QTypes            []RRType        `json:"qtypes,omitempty"`
	Transports        []Transport     `json:"transports,omitempty"`
}

// ChaosSelector decides whether a matching policy triggers and how.
type ChaosSelector struct {
	Mode        SelectorMode  `json:"mode"`
	Seed        string        `json:"seed,omitempty"`
	Probability float64       `json:"probability"`
	TimeBucket  time.Duration `json:"timeBucket,omitempty"`
	Revision    Revision      `json:"revision,omitempty"`
	EveryNth    int           `json:"everyNth,omitempty"`
	SamplingKey string        `json:"samplingKey,omitempty"`
	Period      time.Duration `json:"period,omitempty"`
	Unhealthy   time.Duration `json:"unhealthy,omitempty"`
	PhaseOffset time.Duration `json:"phaseOffset,omitempty"`
}

// ChaosOutcome is one weighted alternative of ordered actions.
type ChaosOutcome struct {
	ID      string        `json:"id"`
	Weight  float64       `json:"weight"`
	Actions []ChaosAction `json:"actions"`
}

// ChaosAction is one effect. Invalid type/phase combinations fail at compile.
type ChaosAction struct {
	Type         string        `json:"type"`
	Phase        string        `json:"phase,omitempty"`
	Distribution string        `json:"distribution,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	Min          time.Duration `json:"min,omitempty"`
	Max          time.Duration `json:"max,omitempty"`
	Value        string        `json:"value,omitempty"`
	EDE          *EDE          `json:"ede,omitempty"`
	TTL          time.Duration `json:"ttl,omitempty"`
	Values       []string      `json:"values,omitempty"`
	Limit        int           `json:"limit,omitempty"`
	UpstreamID   UpstreamID    `json:"upstreamId,omitempty"`
	Hold         time.Duration `json:"hold,omitempty"`
}

// EDE is an optional Extended DNS Error attached to an injected RCODE.
type EDE struct {
	Code int    `json:"code"`
	Text string `json:"text,omitempty"`
}

// ChaosBudget is a policy-scoped request; global SafetySpec still wins.
type ChaosBudget struct {
	MaxDelay       time.Duration `json:"maxDelay,omitempty"`
	MaxConcurrency int           `json:"maxConcurrency,omitempty"`
	MaxRate        float64       `json:"maxRate,omitempty"`
	MaxFrequency   float64       `json:"maxFrequency,omitempty"`
}

// ChaosActivation is the value of OpUpdate + TargetChaosActivation.
type ChaosActivation struct {
	Enabled   bool       `json:"enabled"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}
