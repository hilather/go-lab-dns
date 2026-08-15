package app

import (
	"encoding/json"
	"net/netip"
	"time"

	"github.com/hilather/go-lab-dns/internal/buildinfo"
	"github.com/hilather/go-lab-dns/internal/model"
)

// ChangeIn is the shared plan/apply envelope.
type ChangeIn struct {
	ExpectedRevision model.Revision
	IdempotencyKey   string
	Reason           string
	Ticket           string
	Mode             string // plan | apply; Plan() and Apply() ignore this and follow the method
	Operations       []model.Operation
}

// ValidateIn validates a candidate document and/or operations. expectedRevision
// is not required: this is inspection, not a write.
type ValidateIn struct {
	State      *model.State
	Operations []model.Operation
}

// ResetIn is the privileged bootstrap reread. expectedRevision is not required.
type ResetIn struct {
	Reason string
	Ticket string
}

// Plan is the dry-run result of validate/plan (and the body of apply).
type Plan struct {
	PreviousRevision  model.Revision
	CandidateRevision model.Revision
	Drifted           bool
	Diff              []DiffEntry
	Impact            Impact
	Warnings          []Warning
	Operations        []model.Operation
	Auth              AuthDecision
}

// ApplyResult is a committed (or emergency) mutation result.
type ApplyResult struct {
	Plan
	Applied      bool
	Generation   model.Generation
	AuditEventID string
}

// ExportFormat selects canonical YAML or JSON. Comments are never preserved.
type ExportFormat string

const (
	ExportYAML ExportFormat = "yaml"
	ExportJSON ExportFormat = "json"
)

// Export is canonical desired state plus drift material.
type Export struct {
	Format             ExportFormat
	Body               []byte
	Revision           model.Revision
	BootstrapRevision  model.Revision
	Drifted            bool
	BootstrapToRuntime []model.Operation
	HumanDiff          string
	DeploymentGuidance string
}

// StateView is GET /v1/state. Canonical is a copy; mutating it cannot
// affect the live snapshot.
type StateView struct {
	BootstrapRevision model.Revision
	RuntimeRevision   model.Revision
	Generation        model.Generation
	Drifted           bool
	LoadedAt          time.Time
	Canonical         *model.State
}

// Status is the agent-readable process DTO.
type Status struct {
	Version   buildinfo.Info
	Revisions RevisionView
	Listeners []ListenerStatus
	Cache     CacheSummary
	Upstreams []UpstreamStatus
	Chaos     ChaosRuntimeStatus
	Warnings  []Warning
}

// RevisionView is bootstrap vs runtime identity.
type RevisionView struct {
	BootstrapRevision model.Revision
	RuntimeRevision   model.Revision
	Generation        model.Generation
	Drifted           bool
	LoadedAt          time.Time
}

// ListenerStatus is one bound (or configured) listener.
type ListenerStatus struct {
	Name    string
	Address string
}

// CacheSummary is process-cache bounds and counters. Entries are not in Snapshot.
type CacheSummary struct {
	Enabled    bool
	MaxEntries int
	Entries    int
	Hits       int
	Misses     int
	Evicts     int
}

// UpstreamStatus is one configured upstream plus optional runtime health.
type UpstreamStatus struct {
	ID        model.UpstreamID
	PoolID    model.PoolID
	Endpoint  string
	Transport model.Transport
	Healthy   bool
}

// ChaosRuntimeStatus is YAML plus the emergency-off bit.
type ChaosRuntimeStatus struct {
	Enabled           bool
	EmergencyDisabled bool
	ActivePolicies    int
	NearestExpiry     *time.Time
}

// Warning is a bounded, stable-coded note.
type Warning struct {
	Code    string
	Message string
}

// DiffEntry is one canonical-path change. Paths are sorted in plans.
type DiffEntry struct {
	Path   string          `json:"path"`
	Op     string          `json:"op"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

// Impact is the agent-first summary of a candidate.
type Impact struct {
	Names                 []model.Name
	Zones                 []model.ZoneID
	WildcardCoverage      bool
	AuthoritativeMisses   bool
	ClientGroups          []model.ClientGroupID
	ForwardingChanged     bool
	ChaosPolicies         []ChaosImpact
	CompatibilityWarnings []string
	RequiredPermissions   []string
	SuggestedProbes       []string
}

// ChaosImpact names a policy whose enabled/expiry state changed.
type ChaosImpact struct {
	ID        model.PolicyID
	Enabled   bool
	ExpiresAt *time.Time
}

// AuthDecision is a placeholder until SEC-001. Allowed is always true here.
type AuthDecision struct {
	Allowed bool
	Scopes  []string
}

// CapabilityView lists frozen capability names for discovery.
type CapabilityView struct {
	Capabilities []CapabilityInfo
}

// CapabilityInfo is one registry discovery row (tool name, or health.* id).
type CapabilityInfo struct {
	Name        string
	Version     string
	Description string
	Mutating    bool
	Idempotent  bool
}

// Page is opaque-cursor pagination. Empty cursor starts at the beginning.
type Page struct {
	Limit  int
	Cursor string
}

// ZoneList is a page of canonical zones (copies).
type ZoneList struct {
	Zones      []model.Zone
	NextCursor string
}

// RecordList is a page of canonical records (copies).
type RecordList struct {
	Records    []model.Record
	NextCursor string
}

// ResolveIn is a management-plane lookup. ApplyChaos is reserved for CHA-002.
type ResolveIn struct {
	Name        model.Name
	Type        model.RRType
	Class       model.RRClass
	Client      netip.Addr
	ClientGroup model.ClientGroupID
	Transport   model.Transport
	RD          bool
	CD          bool
	UseCache    bool
	ApplyChaos  bool
}

// ResolveOut is a local resolver answer against the active snapshot.
type ResolveOut struct {
	Result model.Result
}

// ExplainOut reuses resolver Result.Explanation.
type ExplainOut struct {
	Result      model.Result
	Explanation *model.Explanation
}

// FlushIn selects a cache flush. First GA flushes the whole process cache;
// name selectors are not implemented here.
type FlushIn struct {
	All bool
}

// SimulateIn is a side-effect-free chaos decision request.
type SimulateIn struct {
	Name         model.Name
	Type         model.RRType
	Class        model.RRClass
	Client       netip.Addr
	ClientGroup  model.ClientGroupID
	Transport    model.Transport
	ZoneID       model.ZoneID
	ForwardingID model.PolicyID
	PolicyID     model.PolicyID
	PolicyIDs    []model.PolicyID
	Nonce        string
	Phase        string
	Base         *model.Result
}

// SimulateOut is the explained decision. It never sleeps or consumes budgets.
type SimulateOut struct {
	Algorithm string
	Disabled  bool
	Reason    string
	Triggered bool
	Decisions []ChaosDecision
}

// ChaosDecision is one evaluated policy in a simulation.
type ChaosDecision struct {
	PolicyID   model.PolicyID
	OutcomeID  string
	Triggered  bool
	SkipReason string
	DigestHex  string
}

// ActivationIn activates or deactivates a policy via Apply.
type ActivationIn struct {
	PolicyID         model.PolicyID
	ExpectedRevision model.Revision
	IdempotencyKey   string
	Reason           string
	ExpiresAt        *time.Time
}

// ExpiryIn is accepted so the method exists; SetChaosExpiry is a stub.
type ExpiryIn struct {
	PolicyID         model.PolicyID
	ExpectedRevision model.Revision
	IdempotencyKey   string
	Reason           string
	ExpiresAt        *time.Time
}

// EmergencyIn flips the snapshot EmergencyChaosOff bit.
type EmergencyIn struct {
	Reason string
}

// AuditQuery lists recent in-memory events.
type AuditQuery struct {
	Limit int
}

// AuditList is a newest-first page of the ring.
type AuditList struct {
	Events []AuditEvent
}

// AuditEvent is one mutation or emergency record. The ring is process-local.
type AuditEvent struct {
	ID         string
	Time       time.Time
	ActorID    string
	Capability string
	Reason     string
	Ticket     string
	Revision   model.Revision
	Previous   model.Revision
}
