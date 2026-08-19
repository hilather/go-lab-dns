package rest

import (
	"encoding/json"
	"net/netip"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/buildinfo"
	"github.com/hilather/go-lab-dns/internal/model"
)

type healthResponse struct {
	Status string `json:"status"`
}

type sessionResponse struct {
	CSRF  string           `json:"csrf"`
	Actor sessionActorJSON `json:"actor"`
}

type sessionActorJSON struct {
	ID     string   `json:"id"`
	Class  string   `json:"class"`
	Role   string   `json:"role,omitempty"`
	Scopes []string `json:"scopes"`
	Groups []string `json:"groups,omitempty"`
}

type versionResponse struct {
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

type capabilityViewResponse struct {
	Capabilities []capabilityInfo `json:"capabilities"`
}

type capabilityInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Mutating    bool   `json:"mutating"`
	Idempotent  bool   `json:"idempotent"`
}

type statusResponse struct {
	Version   versionResponse  `json:"version"`
	Ready     bool             `json:"ready"`
	Degraded  bool             `json:"degraded"`
	Revisions revisionViewJSON `json:"revisions"`
	Listeners []listenerJSON   `json:"listeners"`
	Cache     cacheSummaryJSON `json:"cache"`
	Upstreams []upstreamJSON   `json:"upstreams"`
	Chaos     chaosStatusJSON  `json:"chaos"`
	Warnings  []warningJSON    `json:"warnings,omitempty"`
}

type revisionViewJSON struct {
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
	BootstrapRevision string          `json:"bootstrapRevision"`
	RuntimeRevision   string          `json:"runtimeRevision"`
	Generation        uint64          `json:"generation"`
	Drifted           bool            `json:"drifted"`
	LoadedAt          string          `json:"loadedAt,omitempty"`
	Canonical         json.RawMessage `json:"canonical"`
}

type zoneListJSON struct {
	Zones      []model.Zone `json:"zones"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

type recordListJSON struct {
	Records    []model.Record `json:"records"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

type resolveRequest struct {
	Name          string              `json:"name"`
	Type          string              `json:"type"`
	ClientContext *clientContextJSON  `json:"clientContext"`
	Options       *resolveOptionsJSON `json:"options"`
}

type clientContextJSON struct {
	ClientGroup string `json:"clientGroup"`
	Transport   string `json:"transport"`
	Client      string `json:"client"`
	RD          bool   `json:"rd"`
	CD          bool   `json:"cd"`
}

type resolveOptionsJSON struct {
	UseCache   *bool `json:"useCache"`
	ApplyChaos *bool `json:"applyChaos"`
}

type changeRequest struct {
	ExpectedRevision string            `json:"expectedRevision"`
	IdempotencyKey   string            `json:"idempotencyKey"`
	Reason           string            `json:"reason"`
	Ticket           string            `json:"ticket"`
	Mode             string            `json:"mode"`
	Operations       []model.Operation `json:"operations"`
	// State is decoded with config.DecodeJSON so duration strings match GET /v1/state.
	State json.RawMessage `json:"state"`
}

type resetRequest struct {
	Reason string `json:"reason"`
	Ticket string `json:"ticket"`
}

type flushRequest struct {
	All bool `json:"all"`
}

type emergencyRequest struct {
	Reason string `json:"reason"`
}

type activationRequest struct {
	ExpectedRevision string `json:"expectedRevision"`
	IdempotencyKey   string `json:"idempotencyKey"`
	Reason           string `json:"reason"`
	ExpiresAt        string `json:"expiresAt"`
}

type simulateRequest struct {
	Name          string             `json:"name"`
	Type          string             `json:"type"`
	ClientContext *clientContextJSON `json:"clientContext"`
	ZoneID        string             `json:"zoneId"`
	ForwardingID  string             `json:"forwardingId"`
	PolicyID      string             `json:"policyId"`
	PolicyIDs     []string           `json:"policyIds"`
	Nonce         string             `json:"nonce"`
	Phase         string             `json:"phase"`
}

type exportJSON struct {
	Format             string            `json:"format"`
	Revision           string            `json:"revision"`
	BootstrapRevision  string            `json:"bootstrapRevision"`
	Drifted            bool              `json:"drifted"`
	Body               json.RawMessage   `json:"body"`
	BootstrapToRuntime []model.Operation `json:"bootstrapToRuntime,omitempty"`
	HumanDiff          string            `json:"humanDiff,omitempty"`
	DeploymentGuidance string            `json:"deploymentGuidance,omitempty"`
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
	ActorClass string `json:"actorClass,omitempty"`
	Transport  string `json:"transport,omitempty"`
	Capability string `json:"capability,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Ticket     string `json:"ticket,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Previous   string `json:"previous,omitempty"`
	Result     string `json:"result,omitempty"`
	ErrorCode  string `json:"errorCode,omitempty"`
}

func fromVersion(info *buildinfo.Info) versionResponse {
	if info == nil {
		return versionResponse{}
	}
	return versionResponse{
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

func fromCapabilities(v *app.CapabilityView) capabilityViewResponse {
	out := capabilityViewResponse{}
	if v == nil {
		return out
	}
	out.Capabilities = make([]capabilityInfo, len(v.Capabilities))
	for i, c := range v.Capabilities {
		out.Capabilities[i] = capabilityInfo{
			Name: c.Name, Version: c.Version, Description: c.Description,
			Mutating: c.Mutating, Idempotent: c.Idempotent,
		}
	}
	return out
}

func fromStatus(st *app.Status) statusResponse {
	if st == nil {
		return statusResponse{}
	}
	out := statusResponse{
		Version:   fromVersion(&st.Version),
		Ready:     st.Ready,
		Degraded:  st.Degraded,
		Revisions: fromRevision(st.Revisions),
		Cache: cacheSummaryJSON{
			Enabled: st.Cache.Enabled, MaxEntries: st.Cache.MaxEntries,
			Entries: st.Cache.Entries, Hits: st.Cache.Hits, Misses: st.Cache.Misses, Evicts: st.Cache.Evicts,
		},
		Chaos:    fromChaosStatus(&st.Chaos),
		Warnings: fromWarnings(st.Warnings),
	}
	for _, l := range st.Listeners {
		out.Listeners = append(out.Listeners, listenerJSON{Name: l.Name, Address: l.Address})
	}
	for _, u := range st.Upstreams {
		out.Upstreams = append(out.Upstreams, upstreamJSON{
			ID: string(u.ID), PoolID: string(u.PoolID), Endpoint: u.Endpoint,
			Transport: string(u.Transport), Healthy: u.Healthy,
		})
	}
	return out
}

func fromRevision(v app.RevisionView) revisionViewJSON {
	return revisionViewJSON{
		BootstrapRevision: string(v.BootstrapRevision),
		RuntimeRevision:   string(v.RuntimeRevision),
		Generation:        uint64(v.Generation),
		Drifted:           v.Drifted,
		LoadedAt:          rfc3339(v.LoadedAt),
	}
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
		out.Canonical = raw
	}
	return out, nil
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
		ID: e.ID, Time: rfc3339(e.Time), ActorID: e.ActorID, ActorClass: e.ActorClass,
		Transport: e.Transport, Capability: e.Capability,
		Reason: e.Reason, Ticket: e.Ticket, Revision: string(e.Revision), Previous: string(e.Previous),
		Result: e.Result, ErrorCode: e.ErrorCode,
	}
}

func toResolveIn(in resolveRequest) (app.ResolveIn, error) {
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
	// Management resolve defaults to not consuming live chaos.
	if in.Options != nil && in.Options.UseCache != nil {
		out.UseCache = *in.Options.UseCache
	}
	if in.Options != nil && in.Options.ApplyChaos != nil {
		out.ApplyChaos = *in.Options.ApplyChaos
	}
	return out, nil
}
