package capabilities

// ID is a frozen capability identifier. Renames are a public-surface change.
type ID string

// First-GA capability IDs. Order matches the implementation-design table.
const (
	HealthLive         ID = "health.live"
	HealthReady        ID = "health.ready"
	Version            ID = "version"
	CapabilitiesID     ID = "capabilities"
	Status             ID = "status"
	SchemaConfig       ID = "schema.config"
	StateGet           ID = "state.get"
	StateValidate      ID = "state.validate"
	ChangePlan         ID = "change.plan"
	ChangeApply        ID = "change.apply"
	StateExport        ID = "state.export"
	StateReset         ID = "state.reset"
	Zones              ID = "zones"
	Records            ID = "records"
	Resolve            ID = "resolve"
	ResolveExplain     ID = "resolve.explain"
	ForwardingPolicies ID = "forwarding.policies"
	UpstreamPools      ID = "upstream.pools"
	UpstreamsStatus    ID = "upstreams.status"
	CacheStatus        ID = "cache.status"
	CacheFlush         ID = "cache.flush"
	ChaosStatus        ID = "chaos.status"
	ChaosPolicies      ID = "chaos.policies"
	ChaosSimulate      ID = "chaos.simulate"
	ChaosActivate      ID = "chaos.activate"
	ChaosSetExpiry     ID = "chaos.set_expiry"
	ChaosEmergency     ID = "chaos.emergency"
	AuditList          ID = "audit.list"
	AuditGet           ID = "audit.get"
	DocsDNSSemantics   ID = "docs.dns-semantics"
	DocsChaosSafety    ID = "docs.chaos-safety"
)

// VersionTag is the first-GA capability schema version embedded on every row.
const VersionTag = "v1"
