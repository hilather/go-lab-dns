package mcp

import (
	"encoding/json"
	"net/netip"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/buildinfo"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

type emptyIn struct{}

type pageIn struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type idIn struct {
	ID string `json:"id"`
}

type zoneIDIn struct {
	ZoneID string `json:"zoneId"`
}

type recordGetIn struct {
	ZoneID   string `json:"zoneId"`
	RecordID string `json:"recordId"`
}

type recordsListIn struct {
	ZoneID string `json:"zoneId"`
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type docsIn struct {
	ID string `json:"id"`
}

type exportIn struct {
	Format string `json:"format,omitempty"`
}

type changeIn struct {
	ExpectedRevision string           `json:"expectedRevision,omitempty"`
	IdempotencyKey   string           `json:"idempotencyKey,omitempty"`
	Reason           string           `json:"reason,omitempty"`
	Ticket           string           `json:"ticket,omitempty"`
	Mode             string           `json:"mode,omitempty"`
	Operations       []map[string]any `json:"operations,omitempty"`
}

type validateIn struct {
	State      map[string]any   `json:"state,omitempty"`
	Operations []map[string]any `json:"operations,omitempty"`
}

type resetIn struct {
	Reason string `json:"reason,omitempty"`
	Ticket string `json:"ticket,omitempty"`
}

type flushIn struct {
	All bool `json:"all,omitempty"`
}

type resolveIn struct {
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	ClientContext *clientContext `json:"clientContext,omitempty"`
	Options       *resolveOpts   `json:"options,omitempty"`
}

type clientContext struct {
	ClientGroup string `json:"clientGroup,omitempty"`
	Transport   string `json:"transport,omitempty"`
	Client      string `json:"client,omitempty"`
	RD          bool   `json:"rd,omitempty"`
	CD          bool   `json:"cd,omitempty"`
}

type resolveOpts struct {
	UseCache   *bool `json:"useCache,omitempty"`
	ApplyChaos *bool `json:"applyChaos,omitempty"`
}

type simulateIn struct {
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	ClientContext *clientContext `json:"clientContext,omitempty"`
	ZoneID        string         `json:"zoneId,omitempty"`
	ForwardingID  string         `json:"forwardingId,omitempty"`
	PolicyID      string         `json:"policyId,omitempty"`
	PolicyIDs     []string       `json:"policyIds,omitempty"`
	Nonce         string         `json:"nonce,omitempty"`
	Phase         string         `json:"phase,omitempty"`
}

type activationIn struct {
	ID               string `json:"id"`
	ExpectedRevision string `json:"expectedRevision,omitempty"`
	IdempotencyKey   string `json:"idempotencyKey,omitempty"`
	Reason           string `json:"reason,omitempty"`
	ExpiresAt        string `json:"expiresAt,omitempty"`
}

type emergencyIn struct {
	Reason string `json:"reason,omitempty"`
}

type auditQueryIn struct {
	Limit int `json:"limit,omitempty"`
}

func (in changeIn) toChange() (app.ChangeIn, error) {
	ops, err := decodeOperations(in.Operations)
	if err != nil {
		return app.ChangeIn{}, err
	}
	return app.ChangeIn{
		ExpectedRevision: model.Revision(in.ExpectedRevision),
		IdempotencyKey:   in.IdempotencyKey,
		Reason:           in.Reason,
		Ticket:           in.Ticket,
		Mode:             in.Mode,
		Operations:       ops,
	}, nil
}

func (in validateIn) toValidate() (app.ValidateIn, error) {
	st, err := decodeCandidateState(in.State)
	if err != nil {
		return app.ValidateIn{}, err
	}
	ops, err := decodeOperations(in.Operations)
	if err != nil {
		return app.ValidateIn{}, err
	}
	return app.ValidateIn{State: st, Operations: ops}, nil
}

func decodeOperations(in []map[string]any) ([]model.Operation, error) {
	if len(in) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, domainerr.ValidationFailed("invalid operations")
	}
	var ops []model.Operation
	if err := json.Unmarshal(raw, &ops); err != nil {
		return nil, domainerr.ValidationFailed("invalid operations")
	}
	return ops, nil
}

func decodeCandidateState(in map[string]any) (*model.State, error) {
	if len(in) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return config.DecodeJSON(raw)
}

func (in resolveIn) toResolve() (app.ResolveIn, error) {
	out := app.ResolveIn{
		Name: model.Name(in.Name),
		Type: model.RRType(in.Type),
	}
	if in.ClientContext != nil {
		out.ClientGroup = model.ClientGroupID(in.ClientContext.ClientGroup)
		out.Transport = model.Transport(in.ClientContext.Transport)
		out.RD = in.ClientContext.RD
		out.CD = in.ClientContext.CD
		if in.ClientContext.Client != "" {
			addr, err := netip.ParseAddr(in.ClientContext.Client)
			if err != nil {
				return out, err
			}
			out.Client = addr
		}
	}
	if in.Options != nil && in.Options.UseCache != nil {
		out.UseCache = *in.Options.UseCache
	}
	if in.Options != nil && in.Options.ApplyChaos != nil {
		out.ApplyChaos = *in.Options.ApplyChaos
	}
	return out, nil
}

func (in simulateIn) toSimulate() (app.SimulateIn, error) {
	out := app.SimulateIn{
		Name:         model.Name(in.Name),
		Type:         model.RRType(in.Type),
		ZoneID:       model.ZoneID(in.ZoneID),
		ForwardingID: model.PolicyID(in.ForwardingID),
		PolicyID:     model.PolicyID(in.PolicyID),
		Nonce:        in.Nonce,
		Phase:        in.Phase,
	}
	for _, id := range in.PolicyIDs {
		out.PolicyIDs = append(out.PolicyIDs, model.PolicyID(id))
	}
	if in.ClientContext != nil {
		out.ClientGroup = model.ClientGroupID(in.ClientContext.ClientGroup)
		out.Transport = model.Transport(in.ClientContext.Transport)
		if in.ClientContext.Client != "" {
			addr, err := netip.ParseAddr(in.ClientContext.Client)
			if err != nil {
				return out, err
			}
			out.Client = addr
		}
	}
	return out, nil
}

func (in activationIn) toActivation() (app.ActivationIn, error) {
	exp, err := parseTimePtr(in.ExpiresAt)
	if err != nil {
		return app.ActivationIn{}, err
	}
	return app.ActivationIn{
		PolicyID:         model.PolicyID(in.ID),
		ExpectedRevision: model.Revision(in.ExpectedRevision),
		IdempotencyKey:   in.IdempotencyKey,
		Reason:           in.Reason,
		ExpiresAt:        exp,
	}, nil
}

func (in activationIn) toExpiry() (app.ExpiryIn, error) {
	exp, err := parseTimePtr(in.ExpiresAt)
	if err != nil {
		return app.ExpiryIn{}, err
	}
	return app.ExpiryIn{
		PolicyID:         model.PolicyID(in.ID),
		ExpectedRevision: model.Revision(in.ExpectedRevision),
		IdempotencyKey:   in.IdempotencyKey,
		Reason:           in.Reason,
		ExpiresAt:        exp,
	}, nil
}

func (in pageIn) page() app.Page {
	return app.Page{Limit: in.Limit, Cursor: in.Cursor}
}

// Output DTOs twin REST camelCase. Do not marshal untagged app structs.
type versionJSON struct {
	Version   string           `json:"version"`
	Commit    string           `json:"commit"`
	BuildTime string           `json:"buildTime"`
	Protocols versionProtocols `json:"protocols"`
}

type versionProtocols struct {
	ConfigAPI string `json:"configAPI"`
	REST      string `json:"rest"`
	MCP       string `json:"mcp"`
	Chaos     string `json:"chaos"`
}

type capabilityViewJSON struct {
	Capabilities []capabilityInfoJSON `json:"capabilities"`
}

type capabilityInfoJSON struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Mutating    bool   `json:"mutating"`
	Idempotent  bool   `json:"idempotent"`
}

type statusJSON struct {
	Version   versionJSON      `json:"version"`
	Revisions revisionJSON     `json:"revisions"`
	Listeners []listenerJSON   `json:"listeners"`
	Cache     cacheSummaryJSON `json:"cache"`
	Upstreams []upstreamJSON   `json:"upstreams"`
	Chaos     chaosStatusJSON  `json:"chaos"`
	Warnings  []warningJSON    `json:"warnings,omitempty"`
}

type revisionJSON struct {
	BootstrapRevision string `json:"bootstrapRevision"`
	RuntimeRevision   string `json:"runtimeRevision"`
	Generation        uint64 `json:"generation"`
	Drifted           bool   `json:"drifted"`
	LoadedAt          string `json:"loadedAt,omitempty"`
}

type listenerJSON struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type cacheSummaryJSON struct {
	Enabled    bool `json:"enabled"`
	MaxEntries int  `json:"maxEntries"`
	Entries    int  `json:"entries"`
	Hits       int  `json:"hits"`
	Misses     int  `json:"misses"`
	Evicts     int  `json:"evicts"`
}

type upstreamJSON struct {
	ID        string `json:"id"`
	PoolID    string `json:"poolId"`
	Endpoint  string `json:"endpoint"`
	Transport string `json:"transport"`
	Healthy   bool   `json:"healthy"`
}

type chaosStatusJSON struct {
	Enabled           bool   `json:"enabled"`
	EmergencyDisabled bool   `json:"emergencyDisabled"`
	ActivePolicies    int    `json:"activePolicies"`
	NearestExpiry     string `json:"nearestExpiry,omitempty"`
}

type warningJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type stateViewJSON struct {
	BootstrapRevision string `json:"bootstrapRevision"`
	RuntimeRevision   string `json:"runtimeRevision"`
	Generation        uint64 `json:"generation"`
	Drifted           bool   `json:"drifted"`
	LoadedAt          string `json:"loadedAt,omitempty"`
	Canonical         any    `json:"canonical,omitempty"`
}

type zoneListJSON struct {
	Zones      []model.Zone `json:"zones"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

type recordListJSON struct {
	Records    []model.Record `json:"records"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

type planJSON struct {
	PreviousRevision  string            `json:"previousRevision"`
	CandidateRevision string            `json:"candidateRevision"`
	Drifted           bool              `json:"drifted"`
	Diff              []app.DiffEntry   `json:"diff,omitempty"`
	Impact            impactJSON        `json:"impact"`
	Warnings          []warningJSON     `json:"warnings,omitempty"`
	Operations        []model.Operation `json:"operations,omitempty"`
	Auth              authJSON          `json:"auth"`
}

type applyJSON struct {
	planJSON
	Applied      bool   `json:"applied"`
	Generation   uint64 `json:"generation"`
	AuditEventID string `json:"auditEventId,omitempty"`
}

type impactJSON struct {
	Names                 []string      `json:"names,omitempty"`
	Zones                 []string      `json:"zones,omitempty"`
	WildcardCoverage      bool          `json:"wildcardCoverage"`
	AuthoritativeMisses   bool          `json:"authoritativeMisses"`
	ClientGroups          []string      `json:"clientGroups,omitempty"`
	ForwardingChanged     bool          `json:"forwardingChanged"`
	ChaosPolicies         []chaosImpact `json:"chaosPolicies,omitempty"`
	CompatibilityWarnings []string      `json:"compatibilityWarnings,omitempty"`
	RequiredPermissions   []string      `json:"requiredPermissions,omitempty"`
	SuggestedProbes       []string      `json:"suggestedProbes,omitempty"`
}

type chaosImpact struct {
	ID        string `json:"id"`
	Enabled   bool   `json:"enabled"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

type authJSON struct {
	Allowed bool     `json:"allowed"`
	Scopes  []string `json:"scopes,omitempty"`
}

type simulateOutJSON struct {
	Algorithm string              `json:"algorithm"`
	Disabled  bool                `json:"disabled"`
	Reason    string              `json:"reason,omitempty"`
	Triggered bool                `json:"triggered"`
	Decisions []chaosDecisionJSON `json:"decisions,omitempty"`
}

type chaosDecisionJSON struct {
	PolicyID   string `json:"policyId"`
	OutcomeID  string `json:"outcomeId,omitempty"`
	Triggered  bool   `json:"triggered"`
	SkipReason string `json:"skipReason,omitempty"`
	DigestHex  string `json:"digestHex,omitempty"`
}

type auditListJSON struct {
	Events []auditEventJSON `json:"events"`
}

type auditEventJSON struct {
	ID         string `json:"id"`
	Time       string `json:"time"`
	ActorID    string `json:"actorId,omitempty"`
	Capability string `json:"capability,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Ticket     string `json:"ticket,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Previous   string `json:"previous,omitempty"`
}

func fromVersion(info *buildinfo.Info) versionJSON {
	if info == nil {
		return versionJSON{}
	}
	return versionJSON{
		Version:   info.Version,
		Commit:    info.Commit,
		BuildTime: info.BuildTime,
		Protocols: versionProtocols{
			ConfigAPI: info.Protocols.ConfigAPI,
			REST:      info.Protocols.REST,
			MCP:       info.Protocols.MCP,
			Chaos:     info.Protocols.Chaos,
		},
	}
}

func fromCapabilities(v *app.CapabilityView) capabilityViewJSON {
	out := capabilityViewJSON{}
	if v == nil {
		return out
	}
	out.Capabilities = make([]capabilityInfoJSON, len(v.Capabilities))
	for i, c := range v.Capabilities {
		out.Capabilities[i] = capabilityInfoJSON{
			Name: c.Name, Version: c.Version, Description: c.Description,
			Mutating: c.Mutating, Idempotent: c.Idempotent,
		}
	}
	return out
}

func fromStatus(st *app.Status) statusJSON {
	if st == nil {
		return statusJSON{}
	}
	out := statusJSON{
		Version:   fromVersion(&st.Version),
		Revisions: fromRevision(st.Revisions),
		Cache:     fromCache(&st.Cache),
		Chaos:     fromChaosStatus(&st.Chaos),
		Warnings:  fromWarnings(st.Warnings),
	}
	for _, l := range st.Listeners {
		out.Listeners = append(out.Listeners, listenerJSON{Name: l.Name, Address: l.Address})
	}
	out.Upstreams = fromUpstreams(st.Upstreams)
	return out
}

func fromRevision(v app.RevisionView) revisionJSON {
	return revisionJSON{
		BootstrapRevision: string(v.BootstrapRevision),
		RuntimeRevision:   string(v.RuntimeRevision),
		Generation:        uint64(v.Generation),
		Drifted:           v.Drifted,
		LoadedAt:          rfc3339(v.LoadedAt),
	}
}

func fromCache(st *app.CacheSummary) cacheSummaryJSON {
	if st == nil {
		return cacheSummaryJSON{}
	}
	return cacheSummaryJSON{
		Enabled: st.Enabled, MaxEntries: st.MaxEntries, Entries: st.Entries,
		Hits: st.Hits, Misses: st.Misses, Evicts: st.Evicts,
	}
}

func fromUpstreams(in []app.UpstreamStatus) []upstreamJSON {
	out := make([]upstreamJSON, 0, len(in))
	for _, u := range in {
		out = append(out, upstreamJSON{
			ID: string(u.ID), PoolID: string(u.PoolID), Endpoint: u.Endpoint,
			Transport: string(u.Transport), Healthy: u.Healthy,
		})
	}
	return out
}

func fromChaosStatus(st *app.ChaosRuntimeStatus) chaosStatusJSON {
	if st == nil {
		return chaosStatusJSON{}
	}
	out := chaosStatusJSON{
		Enabled: st.Enabled, EmergencyDisabled: st.EmergencyDisabled, ActivePolicies: st.ActivePolicies,
	}
	if st.NearestExpiry != nil {
		out.NearestExpiry = rfc3339(*st.NearestExpiry)
	}
	return out
}

func fromWarnings(in []app.Warning) []warningJSON {
	if len(in) == 0 {
		return nil
	}
	out := make([]warningJSON, len(in))
	for i, w := range in {
		out[i] = warningJSON{Code: w.Code, Message: w.Message}
	}
	return out
}

func fromZoneList(list *app.ZoneList) zoneListJSON {
	out := zoneListJSON{Zones: []model.Zone{}}
	if list == nil {
		return out
	}
	out.Zones = list.Zones
	out.NextCursor = list.NextCursor
	return out
}

func fromRecordList(list *app.RecordList) recordListJSON {
	out := recordListJSON{Records: []model.Record{}}
	if list == nil {
		return out
	}
	out.Records = list.Records
	out.NextCursor = list.NextCursor
	return out
}

func fromPlan(p *app.Plan) planJSON {
	if p == nil {
		return planJSON{}
	}
	return planJSON{
		PreviousRevision:  string(p.PreviousRevision),
		CandidateRevision: string(p.CandidateRevision),
		Drifted:           p.Drifted,
		Diff:              p.Diff,
		Impact:            fromImpact(p.Impact),
		Warnings:          fromWarnings(p.Warnings),
		Operations:        p.Operations,
		Auth:              authJSON{Allowed: p.Auth.Allowed, Scopes: p.Auth.Scopes},
	}
}

func fromApply(r *app.ApplyResult) applyJSON {
	if r == nil {
		return applyJSON{}
	}
	return applyJSON{
		planJSON:     fromPlan(&r.Plan),
		Applied:      r.Applied,
		Generation:   uint64(r.Generation),
		AuditEventID: r.AuditEventID,
	}
}

func fromImpact(in app.Impact) impactJSON {
	out := impactJSON{
		WildcardCoverage:      in.WildcardCoverage,
		AuthoritativeMisses:   in.AuthoritativeMisses,
		ForwardingChanged:     in.ForwardingChanged,
		CompatibilityWarnings: in.CompatibilityWarnings,
		RequiredPermissions:   in.RequiredPermissions,
		SuggestedProbes:       in.SuggestedProbes,
	}
	for _, n := range in.Names {
		out.Names = append(out.Names, string(n))
	}
	for _, z := range in.Zones {
		out.Zones = append(out.Zones, string(z))
	}
	for _, g := range in.ClientGroups {
		out.ClientGroups = append(out.ClientGroups, string(g))
	}
	for _, c := range in.ChaosPolicies {
		ci := chaosImpact{ID: string(c.ID), Enabled: c.Enabled}
		if c.ExpiresAt != nil {
			ci.ExpiresAt = rfc3339(*c.ExpiresAt)
		}
		out.ChaosPolicies = append(out.ChaosPolicies, ci)
	}
	return out
}

func fromSimulate(o *app.SimulateOut) simulateOutJSON {
	if o == nil {
		return simulateOutJSON{}
	}
	out := simulateOutJSON{
		Algorithm: o.Algorithm, Disabled: o.Disabled, Reason: o.Reason, Triggered: o.Triggered,
	}
	for _, d := range o.Decisions {
		out.Decisions = append(out.Decisions, chaosDecisionJSON{
			PolicyID: string(d.PolicyID), OutcomeID: d.OutcomeID, Triggered: d.Triggered,
			SkipReason: d.SkipReason, DigestHex: d.DigestHex,
		})
	}
	return out
}

func fromAuditList(l *app.AuditList) auditListJSON {
	out := auditListJSON{Events: []auditEventJSON{}}
	if l == nil {
		return out
	}
	for _, e := range l.Events {
		out.Events = append(out.Events, fromAuditEvent(&e))
	}
	return out
}

func fromAuditEvent(e *app.AuditEvent) auditEventJSON {
	if e == nil {
		return auditEventJSON{}
	}
	return auditEventJSON{
		ID: e.ID, Time: rfc3339(e.Time), ActorID: e.ActorID, Capability: e.Capability,
		Reason: e.Reason, Ticket: e.Ticket, Revision: string(e.Revision), Previous: string(e.Previous),
	}
}

func fromStateView(v *app.StateView) (stateViewJSON, error) {
	out := stateViewJSON{}
	if v == nil {
		return out, nil
	}
	out.BootstrapRevision = string(v.BootstrapRevision)
	out.RuntimeRevision = string(v.RuntimeRevision)
	out.Generation = uint64(v.Generation)
	out.Drifted = v.Drifted
	out.LoadedAt = rfc3339(v.LoadedAt)
	if v.Canonical != nil {
		raw, err := marshalAPI(v.Canonical)
		if err != nil {
			return out, err
		}
		var tree any
		if err := json.Unmarshal(raw, &tree); err != nil {
			return out, err
		}
		out.Canonical = tree
	}
	return out, nil
}
