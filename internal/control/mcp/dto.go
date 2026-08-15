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

func fromVersion(info *buildinfo.Info) map[string]any {
	if info == nil {
		return map[string]any{}
	}
	return map[string]any{
		"version":   info.Version,
		"commit":    info.Commit,
		"buildTime": info.BuildTime,
		"protocols": map[string]any{
			"configAPI": info.Protocols.ConfigAPI,
			"rest":      info.Protocols.REST,
			"mcp":       info.Protocols.MCP,
			"chaos":     info.Protocols.Chaos,
		},
	}
}

func fromCapabilities(v *app.CapabilityView) map[string]any {
	caps := []map[string]any{}
	if v != nil {
		for _, c := range v.Capabilities {
			caps = append(caps, map[string]any{
				"name": c.Name, "version": c.Version, "description": c.Description,
				"mutating": c.Mutating, "idempotent": c.Idempotent,
			})
		}
	}
	return map[string]any{"capabilities": caps}
}

func fromStatus(st *app.Status) map[string]any {
	if st == nil {
		return map[string]any{}
	}
	v := fromVersion(&st.Version)
	return map[string]any{
		"version":   v,
		"revisions": fromRevision(st.Revisions),
		"listeners": st.Listeners,
		"cache":     st.Cache,
		"upstreams": st.Upstreams,
		"chaos":     st.Chaos,
		"warnings":  st.Warnings,
	}
}

func fromRevision(v app.RevisionView) map[string]any {
	return map[string]any{
		"bootstrapRevision": string(v.BootstrapRevision),
		"runtimeRevision":   string(v.RuntimeRevision),
		"generation":        uint64(v.Generation),
		"drifted":           v.Drifted,
		"loadedAt":          rfc3339(v.LoadedAt),
	}
}

func fromZoneList(list *app.ZoneList) map[string]any {
	zones := []model.Zone{}
	next := ""
	if list != nil {
		zones = list.Zones
		next = list.NextCursor
	}
	return map[string]any{"zones": zones, "nextCursor": next}
}

func fromRecordList(list *app.RecordList) map[string]any {
	recs := []model.Record{}
	next := ""
	if list != nil {
		recs = list.Records
		next = list.NextCursor
	}
	return map[string]any{"records": recs, "nextCursor": next}
}

func fromPlan(p *app.Plan) map[string]any {
	if p == nil {
		return map[string]any{}
	}
	return map[string]any{
		"previousRevision":  string(p.PreviousRevision),
		"candidateRevision": string(p.CandidateRevision),
		"drifted":           p.Drifted,
		"diff":              p.Diff,
		"impact":            p.Impact,
		"warnings":          p.Warnings,
		"operations":        p.Operations,
		"auth":              p.Auth,
	}
}

func fromApply(r *app.ApplyResult) map[string]any {
	if r == nil {
		return map[string]any{}
	}
	out := fromPlan(&r.Plan)
	out["applied"] = r.Applied
	out["generation"] = uint64(r.Generation)
	out["auditEventId"] = r.AuditEventID
	return out
}

func fromStateView(v *app.StateView) (any, error) {
	if v == nil {
		return map[string]any{}, nil
	}
	out := map[string]any{
		"bootstrapRevision": string(v.BootstrapRevision),
		"runtimeRevision":   string(v.RuntimeRevision),
		"generation":        uint64(v.Generation),
		"drifted":           v.Drifted,
		"loadedAt":          rfc3339(v.LoadedAt),
	}
	if v.Canonical != nil {
		raw, err := marshalAPI(v.Canonical)
		if err != nil {
			return nil, err
		}
		var tree any
		if err := json.Unmarshal(raw, &tree); err != nil {
			return nil, err
		}
		out["canonical"] = tree
	}
	return out, nil
}
