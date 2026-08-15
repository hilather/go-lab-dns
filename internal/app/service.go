package app

import (
	"context"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/buildinfo"
	"github.com/hilather/go-lab-dns/internal/model"
)

// Actor is the authenticated caller. Authorization is not enforced in this
// slice; the field is recorded on audit events for later SEC-001.
type Actor = auth.Actor

// Service is the full HTTP-less capability surface. REST and MCP must call
// these methods rather than implementing mutation or query logic.
type Service interface {
	Version(ctx context.Context, actor Actor) (*buildinfo.Info, error)
	Capabilities(ctx context.Context, actor Actor) (*CapabilityView, error)
	Status(ctx context.Context, actor Actor) (*Status, error)
	ConfigSchema(ctx context.Context, actor Actor) ([]byte, error)
	Docs(ctx context.Context, actor Actor, id string) ([]byte, error)

	GetState(ctx context.Context, actor Actor) (*StateView, error)
	Validate(ctx context.Context, actor Actor, in ValidateIn) (*Plan, error)
	Plan(ctx context.Context, actor Actor, in ChangeIn) (*Plan, error)
	Apply(ctx context.Context, actor Actor, in ChangeIn) (*ApplyResult, error)
	Export(ctx context.Context, actor Actor, format ExportFormat) (*Export, error)
	Reset(ctx context.Context, actor Actor, in ResetIn) (*ApplyResult, error)

	ListZones(ctx context.Context, actor Actor, page Page) (*ZoneList, error)
	GetZone(ctx context.Context, actor Actor, id model.ZoneID) (*model.Zone, error)
	ListRecords(ctx context.Context, actor Actor, zone model.ZoneID, page Page) (*RecordList, error)
	GetRecord(ctx context.Context, actor Actor, zone model.ZoneID, id model.RecordID) (*model.Record, error)

	Resolve(ctx context.Context, actor Actor, in ResolveIn) (*ResolveOut, error)
	Explain(ctx context.Context, actor Actor, in ResolveIn) (*ExplainOut, error)

	ListForwardingPolicies(ctx context.Context, actor Actor) ([]model.ForwardingPolicy, error)
	ListUpstreamPools(ctx context.Context, actor Actor) ([]model.UpstreamPool, error)
	UpstreamsStatus(ctx context.Context, actor Actor) ([]UpstreamStatus, error)
	CacheStatus(ctx context.Context, actor Actor) (*CacheSummary, error)
	CacheFlush(ctx context.Context, actor Actor, in FlushIn) error

	ChaosStatus(ctx context.Context, actor Actor) (*ChaosRuntimeStatus, error)
	ListChaosPolicies(ctx context.Context, actor Actor) ([]model.ChaosPolicy, error)
	GetChaosPolicy(ctx context.Context, actor Actor, id model.PolicyID) (*model.ChaosPolicy, error)
	SimulateChaos(ctx context.Context, actor Actor, in SimulateIn) (*SimulateOut, error)
	ActivateChaos(ctx context.Context, actor Actor, in ActivationIn) (*ApplyResult, error)
	DeactivateChaos(ctx context.Context, actor Actor, in ActivationIn) (*ApplyResult, error)
	SetChaosExpiry(ctx context.Context, actor Actor, in ExpiryIn) (*ApplyResult, error)
	EmergencyDisableChaos(ctx context.Context, actor Actor, in EmergencyIn) (*ApplyResult, error)
	EmergencyEnableChaos(ctx context.Context, actor Actor, in EmergencyIn) (*ApplyResult, error)

	QueryAudit(ctx context.Context, actor Actor, in AuditQuery) (*AuditList, error)
	GetAudit(ctx context.Context, actor Actor, id string) (*AuditEvent, error)
}
