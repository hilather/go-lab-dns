package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func (s *Server) dispatch(w http.ResponseWriter, r *http.Request, instance string, actor auth.Actor, rt compiledRoute, params map[string]string) {
	ctx := r.Context()
	if err := ctx.Err(); err != nil {
		s.writeProblem(w, r, instance, domainerr.Internal("request canceled"))
		return
	}
	switch rt.cap.ID {
	case capabilities.HealthLive:
		s.handleHealthLive(w, r)
	case capabilities.HealthReady:
		s.handleHealthReady(w, r, ctx)
	case capabilities.Version:
		s.handleVersion(w, r, instance, ctx, actor)
	case capabilities.CapabilitiesID:
		s.handleCapabilities(w, r, instance, ctx, actor)
	case capabilities.Status:
		s.handleStatus(w, r, instance, ctx, actor)
	case capabilities.SchemaConfig:
		s.handleSchema(w, r, instance, ctx, actor)
	case capabilities.StateGet:
		s.handleGetState(w, r, instance, ctx, actor)
	case capabilities.StateValidate:
		s.handleValidate(w, r, instance, ctx, actor)
	case capabilities.ChangePlan:
		s.handlePlan(w, r, instance, ctx, actor)
	case capabilities.ChangeApply:
		s.handleApply(w, r, instance, ctx, actor)
	case capabilities.StateExport:
		s.handleExport(w, r, instance, ctx, actor)
	case capabilities.StateReset:
		s.handleReset(w, r, instance, ctx, actor)
	case capabilities.Zones:
		if params["zoneId"] == "" {
			s.handleListZones(w, r, instance, ctx, actor)
			return
		}
		s.handleGetZone(w, r, instance, ctx, actor, params["zoneId"])
	case capabilities.Records:
		if params["recordId"] == "" {
			s.handleListRecords(w, r, instance, ctx, actor, params["zoneId"])
			return
		}
		s.handleGetRecord(w, r, instance, ctx, actor, params["zoneId"], params["recordId"])
	case capabilities.Resolve:
		s.handleResolve(w, r, instance, ctx, actor)
	case capabilities.ResolveExplain:
		s.handleExplain(w, r, instance, ctx, actor)
	case capabilities.ForwardingPolicies:
		s.handleForwarding(w, r, instance, ctx, actor)
	case capabilities.UpstreamPools:
		s.handlePools(w, r, instance, ctx, actor)
	case capabilities.UpstreamsStatus:
		s.handleUpstreams(w, r, instance, ctx, actor)
	case capabilities.CacheStatus:
		s.handleCacheStatus(w, r, instance, ctx, actor)
	case capabilities.CacheFlush:
		s.handleCacheFlush(w, r, instance, ctx, actor)
	case capabilities.ChaosStatus:
		s.handleChaosStatus(w, r, instance, ctx, actor)
	case capabilities.ChaosPolicies:
		if params["policyId"] == "" {
			s.handleListChaos(w, r, instance, ctx, actor)
			return
		}
		s.handleGetChaos(w, r, instance, ctx, actor, params["policyId"])
	case capabilities.ChaosSimulate:
		s.handleSimulate(w, r, instance, ctx, actor)
	case capabilities.ChaosActivate:
		if strings.HasSuffix(rt.path, ":deactivate") {
			s.handleDeactivate(w, r, instance, ctx, actor, params["id"])
			return
		}
		s.handleActivate(w, r, instance, ctx, actor, params["id"])
	case capabilities.ChaosSetExpiry:
		s.handleExpire(w, r, instance, ctx, actor, params["id"])
	case capabilities.ChaosEmergency:
		if strings.HasSuffix(rt.path, ":emergency-enable") {
			s.handleEmergency(w, r, instance, ctx, actor, false)
			return
		}
		s.handleEmergency(w, r, instance, ctx, actor, true)
	case capabilities.AuditList:
		s.handleAuditList(w, r, instance, ctx, actor)
	case capabilities.AuditGet:
		s.handleAuditGet(w, r, instance, ctx, actor, params["eventId"])
	case capabilities.DocsDNSSemantics:
		s.handleDocs(w, r, instance, ctx, actor, "dns-semantics")
	case capabilities.DocsChaosSafety:
		s.handleDocs(w, r, instance, ctx, actor, "chaos-safety")
	default:
		s.writeProblem(w, r, instance, domainerr.UnsupportedCapability("unsupported capability"))
	}
}

func (s *Server) handleHealthLive(w http.ResponseWriter, r *http.Request) {
	if !s.isLive() {
		s.writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "down"})
		return
	}
	s.writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	_ = r
}

func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !s.isReady(ctx) {
		s.writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not ready"})
		return
	}
	s.writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	_ = r
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	info, err := s.svc.Version(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromVersion(info))
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	v, err := s.svc.Capabilities(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromCapabilities(v))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	st, err := s.svc.Status(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromStatus(st))
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	b, err := s.svc.ConfigSchema(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeBytes(w, http.StatusOK, "application/schema+json", b)
}

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, id string) {
	b, err := s.svc.Docs(ctx, actor, id)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeBytes(w, http.StatusOK, "text/markdown; charset=utf-8", b)
}

func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	v, err := s.svc.GetState(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	out, err := fromStateView(v)
	if err != nil {
		s.writeProblem(w, r, instance, domainerr.Internal("internal error"))
		return
	}
	if v != nil && v.RuntimeRevision != "" {
		w.Header().Set(headerRevision, string(v.RuntimeRevision))
	}
	s.writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListZones(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	page, err := pageFromQuery(r)
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	list, err := s.svc.ListZones(ctx, actor, page)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, zoneListJSON{Zones: list.Zones, NextCursor: list.NextCursor})
}

func (s *Server) handleGetZone(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, id string) {
	z, err := s.svc.GetZone(ctx, actor, model.ZoneID(id))
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, z)
}

func (s *Server) handleListRecords(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, zone string) {
	page, err := pageFromQuery(r)
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	list, err := s.svc.ListRecords(ctx, actor, model.ZoneID(zone), page)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, recordListJSON{Records: list.Records, NextCursor: list.NextCursor})
}

func (s *Server) handleGetRecord(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, zone, id string) {
	rec, err := s.svc.GetRecord(ctx, actor, model.ZoneID(zone), model.RecordID(id))
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	var in resolveRequest
	if !s.decodeJSON(w, r, instance, &in) {
		return
	}
	ri, err := toResolveIn(in)
	if err != nil {
		s.writeProblem(w, r, instance, domainerr.ValidationFailed("invalid client address",
			domainerr.FieldViolation{Path: "clientContext.client", Code: "invalid_value", Message: "client must be an IP address"}))
		return
	}
	out, err := s.svc.Resolve(ctx, actor, ri)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"result": out.Result})
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	var in resolveRequest
	if !s.decodeJSON(w, r, instance, &in) {
		return
	}
	ri, err := toResolveIn(in)
	if err != nil {
		s.writeProblem(w, r, instance, domainerr.ValidationFailed("invalid client address",
			domainerr.FieldViolation{Path: "clientContext.client", Code: "invalid_value", Message: "client must be an IP address"}))
		return
	}
	out, err := s.svc.Explain(ctx, actor, ri)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"result": out.Result, "explanation": out.Explanation})
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	var in changeRequest
	if !s.decodeJSON(w, r, instance, &in) {
		return
	}
	st, err := decodeCandidateState(in.State)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	plan, err := s.svc.Validate(ctx, actor, app.ValidateIn{State: st, Operations: in.Operations})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromPlan(plan))
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	in, ok := s.readChange(w, r, instance)
	if !ok {
		return
	}
	plan, err := s.svc.Plan(ctx, actor, in)
	if err != nil {
		s.writeMutateErr(w, r, instance, err, string(in.ExpectedRevision))
		return
	}
	s.writeJSON(w, http.StatusOK, fromPlan(plan))
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	in, ok := s.readChange(w, r, instance)
	if !ok {
		return
	}
	res, err := s.svc.Apply(ctx, actor, in)
	if err != nil {
		s.writeMutateErr(w, r, instance, err, string(in.ExpectedRevision))
		return
	}
	if res != nil && res.CandidateRevision != "" {
		w.Header().Set(headerRevision, string(res.CandidateRevision))
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	format := app.ExportYAML
	switch strings.ToLower(r.URL.Query().Get("format")) {
	case "", "yaml", "yml":
		format = app.ExportYAML
	case "json":
		format = app.ExportJSON
	default:
		s.writeProblem(w, r, instance, domainerr.ValidationFailed("unknown export format",
			domainerr.FieldViolation{Path: "format", Code: "invalid_value", Message: "format must be yaml or json"}))
		return
	}
	exp, err := s.svc.Export(ctx, actor, format)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if exp.Revision != "" {
		w.Header().Set(headerRevision, string(exp.Revision))
	}
	if format == app.ExportYAML {
		s.writeBytes(w, http.StatusOK, "application/yaml", exp.Body)
		return
	}
	body := json.RawMessage(exp.Body)
	s.writeJSON(w, http.StatusOK, exportJSON{
		Format:             string(exp.Format),
		Revision:           string(exp.Revision),
		BootstrapRevision:  string(exp.BootstrapRevision),
		Drifted:            exp.Drifted,
		Body:               body,
		BootstrapToRuntime: exp.BootstrapToRuntime,
		HumanDiff:          exp.HumanDiff,
		DeploymentGuidance: exp.DeploymentGuidance,
	})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	var in resetRequest
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	res, err := s.svc.Reset(ctx, actor, app.ResetIn{Reason: in.Reason, Ticket: in.Ticket})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleForwarding(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	pols, err := s.svc.ListForwardingPolicies(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if pols == nil {
		pols = []model.ForwardingPolicy{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"policies": pols})
}

func (s *Server) handlePools(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	pools, err := s.svc.ListUpstreamPools(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if pools == nil {
		pools = []model.UpstreamPool{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"pools": pools})
}

func (s *Server) handleUpstreams(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	ups, err := s.svc.UpstreamsStatus(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	out := make([]upstreamJSON, 0, len(ups))
	for _, u := range ups {
		out = append(out, upstreamJSON{
			ID: string(u.ID), PoolID: string(u.PoolID), Endpoint: u.Endpoint,
			Transport: string(u.Transport), Healthy: u.Healthy,
		})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"upstreams": out})
}

func (s *Server) handleCacheStatus(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	st, err := s.svc.CacheStatus(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, cacheSummaryJSON{
		Enabled: st.Enabled, MaxEntries: st.MaxEntries, Entries: st.Entries,
		Hits: st.Hits, Misses: st.Misses, Evicts: st.Evicts,
	})
}

func (s *Server) handleCacheFlush(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	var in flushRequest
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	if err := s.svc.CacheFlush(ctx, actor, app.FlushIn{All: in.All}); err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleChaosStatus(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	st, err := s.svc.ChaosStatus(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromChaosStatus(st))
}

func (s *Server) handleListChaos(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	pols, err := s.svc.ListChaosPolicies(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if pols == nil {
		pols = []model.ChaosPolicy{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"policies": pols})
}

func (s *Server) handleGetChaos(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, id string) {
	p, err := s.svc.GetChaosPolicy(ctx, actor, model.PolicyID(id))
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleSimulate(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	var in simulateRequest
	if !s.decodeJSON(w, r, instance, &in) {
		return
	}
	si := app.SimulateIn{
		Name: model.Name(in.Name), Type: model.RRType(in.Type),
		ZoneID: model.ZoneID(in.ZoneID), ForwardingID: model.PolicyID(in.ForwardingID),
		PolicyID: model.PolicyID(in.PolicyID), Nonce: in.Nonce, Phase: in.Phase,
	}
	for _, id := range in.PolicyIDs {
		si.PolicyIDs = append(si.PolicyIDs, model.PolicyID(id))
	}
	if in.ClientContext != nil {
		si.ClientGroup = model.ClientGroupID(in.ClientContext.ClientGroup)
		si.Transport = model.Transport(in.ClientContext.Transport)
		if in.ClientContext.Client != "" {
			addr, err := netip.ParseAddr(in.ClientContext.Client)
			if err != nil {
				s.writeProblem(w, r, instance, domainerr.ValidationFailed("invalid client address",
					domainerr.FieldViolation{Path: "clientContext.client", Code: "invalid_value", Message: "client must be an IP address"}))
				return
			}
			si.Client = addr
		}
	}
	out, err := s.svc.SimulateChaos(ctx, actor, si)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromSimulate(out))
}

func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, id string) {
	in, ok := s.readActivation(w, r, instance)
	if !ok {
		return
	}
	in.PolicyID = model.PolicyID(id)
	res, err := s.svc.ActivateChaos(ctx, actor, in)
	if err != nil {
		s.writeMutateErr(w, r, instance, err, string(in.ExpectedRevision))
		return
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleDeactivate(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, id string) {
	in, ok := s.readActivation(w, r, instance)
	if !ok {
		return
	}
	in.PolicyID = model.PolicyID(id)
	res, err := s.svc.DeactivateChaos(ctx, actor, in)
	if err != nil {
		s.writeMutateErr(w, r, instance, err, string(in.ExpectedRevision))
		return
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleExpire(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, id string) {
	var body activationRequest
	if !s.decodeJSONOptional(w, r, instance, &body) {
		return
	}
	exp, err := parseTimePtr(body.ExpiresAt)
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	in := app.ExpiryIn{
		PolicyID:         model.PolicyID(id),
		ExpectedRevision: model.Revision(expectedRevision(r, body.ExpectedRevision)),
		IdempotencyKey:   idempotencyKey(r, body.IdempotencyKey),
		Reason:           body.Reason,
		ExpiresAt:        exp,
	}
	res, err := s.svc.SetChaosExpiry(ctx, actor, in)
	if err != nil {
		s.writeMutateErr(w, r, instance, err, string(in.ExpectedRevision))
		return
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleEmergency(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, disable bool) {
	var in emergencyRequest
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	var (
		res *app.ApplyResult
		err error
	)
	if disable {
		res, err = s.svc.EmergencyDisableChaos(ctx, actor, app.EmergencyIn{Reason: in.Reason})
	} else {
		res, err = s.svc.EmergencyEnableChaos(ctx, actor, app.EmergencyIn{Reason: in.Reason})
	}
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleAuditList(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor) {
	q := app.AuditQuery{}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("invalid limit",
				domainerr.FieldViolation{Path: "limit", Code: "invalid_value", Message: "limit must be a non-negative integer"}))
			return
		}
		q.Limit = n
	}
	list, err := s.svc.QueryAudit(ctx, actor, q)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromAuditList(list))
}

func (s *Server) handleAuditGet(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor auth.Actor, id string) {
	ev, err := s.svc.GetAudit(ctx, actor, id)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromAuditEvent(ev))
}

func (s *Server) readChange(w http.ResponseWriter, r *http.Request, instance string) (app.ChangeIn, bool) {
	var body changeRequest
	if !s.decodeJSON(w, r, instance, &body) {
		return app.ChangeIn{}, false
	}
	return app.ChangeIn{
		ExpectedRevision: model.Revision(expectedRevision(r, body.ExpectedRevision)),
		IdempotencyKey:   idempotencyKey(r, body.IdempotencyKey),
		Reason:           body.Reason,
		Ticket:           body.Ticket,
		Mode:             body.Mode,
		Operations:       body.Operations,
	}, true
}

func (s *Server) readActivation(w http.ResponseWriter, r *http.Request, instance string) (app.ActivationIn, bool) {
	var body activationRequest
	if !s.decodeJSONOptional(w, r, instance, &body) {
		return app.ActivationIn{}, false
	}
	exp, err := parseTimePtr(body.ExpiresAt)
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return app.ActivationIn{}, false
	}
	return app.ActivationIn{
		ExpectedRevision: model.Revision(expectedRevision(r, body.ExpectedRevision)),
		IdempotencyKey:   idempotencyKey(r, body.IdempotencyKey),
		Reason:           body.Reason,
		ExpiresAt:        exp,
	}, true
}

func (s *Server) writeMutateErr(w http.ResponseWriter, r *http.Request, instance string, err error, expected string) {
	de, ok := domainerr.As(asDomain(err))
	if ok && de.Code == domainerr.CodeRevisionConflict {
		s.writeRevisionProblem(w, r, instance, err, expected)
		return
	}
	s.writeProblem(w, r, instance, asDomain(err))
}
