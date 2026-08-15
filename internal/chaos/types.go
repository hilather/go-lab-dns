package chaos

import (
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// AlgorithmID is the frozen deterministic selector. Changing encoding or
// mapping requires a new identifier, not a silent tweak of this value.
const AlgorithmID = "hash-v1"

// Magic is the unprefixed ASCII header of a hash-v1 encoding.
const Magic = "labdns-hash-v1\n"

// SamplingClientBucket is the selector.samplingKey value that puts a
// privacy-safe client-IP bucket into hash-v1 field 7 instead of the group id.
const SamplingClientBucket = "client-bucket"

// Phase is when Decide runs on the query path.
type Phase string

const (
	// PhasePreResolution is before local resolve / upstream exchange.
	PhasePreResolution Phase = "pre-resolution"
	// PhaseResponse is after a base result exists.
	PhaseResponse Phase = "response"
)

// DecisionIn is a classified query. ZoneID and ForwardingID are pre-selected
// by dnsquery; the engine must not rediscover them.
type DecisionIn struct {
	Query           model.Query
	ClientGroupID   model.ClientGroupID
	ZoneID          model.ZoneID
	ForwardingID    model.PolicyID
	Base            *model.Result // nil in pre-resolution
	Phase           Phase
	SimulationNonce string
}

// SimulateIn is a side-effect-free decision request. IDs are pre-classified
// by the caller (app.SimulateChaos may classify first).
type SimulateIn struct {
	Query         model.Query
	ClientGroupID model.ClientGroupID
	ZoneID        model.ZoneID
	ForwardingID  model.PolicyID
	Base          *model.Result
	Phase         Phase
	Nonce         string
	PolicyIDs     []model.PolicyID // optional filter; empty = all
}

// SimulateOut is the explained, non-executing decision.
type SimulateOut struct {
	Algorithm  string
	Disabled   bool
	Reason     string
	Triggered  bool
	Decisions  []PolicyDecision
	Plan       ActionPlan
	BudgetUsed bool // always false; simulation never consumes budgets
}

// ActionPlan is the structured effect list for one Decide call.
// CHA-002 executes it; CHA-001 returns it as a no-op for the data plane.
type ActionPlan struct {
	Algorithm     string
	Disabled      bool
	Reason        string
	Decisions     []PolicyDecision
	Actions       []PlannedAction
	Delay         time.Duration
	EarlyRCode    model.RCode
	TransportHint string
	SkipResolve   bool
	Clamped       []ClampRecord
	Cache         CachePlan
	Upstream      UpstreamPlan
	Pressure      PressurePlan
	EDE           *model.EDE
	Hold          time.Duration
}

// PolicyDecision is one evaluated policy.
type PolicyDecision struct {
	PolicyID   model.PolicyID
	Precedence int
	OutcomeID  string
	Triggered  bool
	SkipReason string
	Hash       HashResult
	Actions    []PlannedAction
}

// PlannedAction is one selected effect after clamping.
type PlannedAction struct {
	PolicyID     model.PolicyID
	OutcomeID    string
	Type         string
	Phase        string
	Distribution string
	Delay        time.Duration
	RCode        string
	Value        string
	Clamped      bool
	EDE          *model.EDE
	TTL          time.Duration
	Min          time.Duration
	Max          time.Duration
	Values       []string
	Limit        int
	UpstreamID   model.UpstreamID
	Hold         time.Duration
	Seed         uint64 // deterministic shuffle/jitter/rotate
	Skip         bool   // planned but not executable (budget)
	Rate         float64
	Concurrency  int
}

// CachePlan is the request-path cache hook derived from selected actions.
type CachePlan struct {
	Bypass     bool
	ForceMiss  bool
	ServeStale bool
	Expire     bool
}

// UpstreamPlan is the request-path forwarder hook derived from selected actions.
type UpstreamPlan struct {
	Delay          time.Duration
	Unavailable    []model.UpstreamID
	Force          model.UpstreamID
	Timeout        bool
	TransportError bool
	Failover       bool
	SyntheticRCode model.RCode
}

// PressurePlan is policy-scoped QPS/concurrency pressure.
type PressurePlan struct {
	PolicyID model.PolicyID
	MaxRate  float64
	MaxConc  int
	OnExceed string
}

// First-GA action value vocabulary. Unknown values are ignored at execution.
const (
	CacheValueBypass    = "bypass"
	CacheValueForceMiss = "force-miss"
	CacheValueExpire    = "expire"
	CacheValueStale     = "stale"

	TTLValueSet    = "set"
	TTLValueClamp  = "clamp"
	TTLValueZero   = "zero"
	TTLValueJitter = "jitter"

	UpstreamValueDelay          = "delay"
	UpstreamValueUnavailable    = "unavailable"
	UpstreamValueForce          = "force"
	UpstreamValueTimeout        = "timeout"
	UpstreamValueTransportError = "transport-error"
	UpstreamValueFailover       = "failover"

	PressureValueDrop     = "drop"
	PressureValueRefused  = "REFUSED"
	PressureValueServFail = "SERVFAIL"

	RCodeNODATA = "NODATA"
)

// ClampRecord explains a safety cap that changed an action.
type ClampRecord struct {
	PolicyID model.PolicyID
	Action   string
	Reason   string
	From     string
	To       string
}

// HashFields is the ten length-prefixed hash-v1 inputs (plus magic).
type HashFields struct {
	Seed        string
	Revision    model.Revision
	PolicyID    model.PolicyID
	QNAME       model.Name
	QTYPE       model.RRType
	ClientGroup string
	Transport   model.Transport
	TimeBucket  string // RFC3339 seconds Z, or empty
	Nonce       string
}

// HashResult is the digest and mapped uniforms.
type HashResult struct {
	DigestHex string
	U0        uint64
	U1        uint64
	P         float64
	W         float64
}
