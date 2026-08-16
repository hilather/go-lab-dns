package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerTools() {
	addTool(s, "dns_version_get", versionDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		info, err := s.svc.Version(ctx, actor)
		if err != nil {
			return nil, err
		}
		return fromVersion(info), nil
	})
	addTool(s, "dns_capabilities_get", capDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		v, err := s.svc.Capabilities(ctx, actor)
		if err != nil {
			return nil, err
		}
		return fromCapabilities(v), nil
	})
	addTool(s, "dns_status_get", statusDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		st, err := s.svc.Status(ctx, actor)
		if err != nil {
			return nil, err
		}
		return fromStatus(st), nil
	})
	addTool(s, "dns_schema_get", schemaDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		b, err := s.svc.ConfigSchema(ctx, actor)
		if err != nil {
			return nil, err
		}
		var doc any
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, domainerr.Internal("internal error")
		}
		return doc, nil
	})
	addTool(s, "dns_docs_get", docsDesc, false, true, func(ctx context.Context, actor auth.Actor, in docsIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id must be dns-semantics or chaos-safety"})
		}
		b, err := s.svc.Docs(ctx, actor, in.ID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": in.ID, "markdown": string(b)}, nil
	})
	addTool(s, "dns_state_get", stateGetDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		v, err := s.svc.GetState(ctx, actor)
		if err != nil {
			return nil, err
		}
		return fromStateView(v)
	})
	addTool(s, "dns_state_validate", validateDesc, false, true, func(ctx context.Context, actor auth.Actor, in validateIn) (any, error) {
		vin, err := in.toValidate()
		if err != nil {
			return nil, asDomain(err)
		}
		p, err := s.svc.Validate(ctx, actor, vin)
		if err != nil {
			return nil, err
		}
		return fromPlan(p), nil
	})
	addTool(s, "dns_change_plan", planDesc, false, true, func(ctx context.Context, actor auth.Actor, in changeIn) (any, error) {
		cin, err := in.toChange()
		if err != nil {
			return nil, err
		}
		p, err := s.svc.Plan(ctx, actor, cin)
		if err != nil {
			return nil, err
		}
		return fromPlan(p), nil
	})
	addTool(s, "dns_change_apply", applyDesc, true, true, func(ctx context.Context, actor auth.Actor, in changeIn) (any, error) {
		cin, err := in.toChange()
		if err != nil {
			return nil, err
		}
		r, err := s.svc.Apply(ctx, actor, cin)
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "dns_state_export", exportDesc, false, true, func(ctx context.Context, actor auth.Actor, in exportIn) (any, error) {
		format := app.ExportYAML
		switch strings.ToLower(in.Format) {
		case "", "yaml", "yml":
			format = app.ExportYAML
		case "json":
			format = app.ExportJSON
		default:
			return nil, domainerr.ValidationFailed("unknown export format",
				domainerr.FieldViolation{Path: "format", Code: "invalid_value", Message: "format must be yaml or json"})
		}
		exp, err := s.svc.Export(ctx, actor, format)
		if err != nil {
			return nil, err
		}
		out := map[string]any{
			"format":             string(exp.Format),
			"revision":           string(exp.Revision),
			"bootstrapRevision":  string(exp.BootstrapRevision),
			"drifted":            exp.Drifted,
			"humanDiff":          exp.HumanDiff,
			"deploymentGuidance": exp.DeploymentGuidance,
			"body":               string(exp.Body),
		}
		if len(exp.BootstrapToRuntime) > 0 {
			out["bootstrapToRuntime"] = exp.BootstrapToRuntime
		}
		return out, nil
	})
	addTool(s, "dns_state_reset", resetDesc, true, false, func(ctx context.Context, actor auth.Actor, in resetIn) (any, error) {
		r, err := s.svc.Reset(ctx, actor, app.ResetIn{Reason: in.Reason, Ticket: in.Ticket})
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "dns_zones_list", zonesListDesc, false, true, func(ctx context.Context, actor auth.Actor, in pageIn) (any, error) {
		list, err := s.svc.ListZones(ctx, actor, in.page())
		if err != nil {
			return nil, err
		}
		return fromZoneList(list), nil
	})
	addTool(s, "dns_zone_get", zoneGetDesc, false, true, func(ctx context.Context, actor auth.Actor, in zoneIDIn) (any, error) {
		if in.ZoneID == "" {
			return nil, domainerr.ValidationFailed("zoneId is required",
				domainerr.FieldViolation{Path: "zoneId", Code: "required", Message: "zoneId is required"})
		}
		return s.svc.GetZone(ctx, actor, model.ZoneID(in.ZoneID))
	})
	addTool(s, "dns_records_list", recordsListDesc, false, true, func(ctx context.Context, actor auth.Actor, in recordsListIn) (any, error) {
		if in.ZoneID == "" {
			return nil, domainerr.ValidationFailed("zoneId is required",
				domainerr.FieldViolation{Path: "zoneId", Code: "required", Message: "zoneId is required"})
		}
		list, err := s.svc.ListRecords(ctx, actor, model.ZoneID(in.ZoneID), app.Page{Limit: in.Limit, Cursor: in.Cursor})
		if err != nil {
			return nil, err
		}
		return fromRecordList(list), nil
	})
	addTool(s, "dns_record_get", recordGetDesc, false, true, func(ctx context.Context, actor auth.Actor, in recordGetIn) (any, error) {
		if in.ZoneID == "" || in.RecordID == "" {
			return nil, domainerr.ValidationFailed("zoneId and recordId are required",
				domainerr.FieldViolation{Path: "recordId", Code: "required", Message: "zoneId and recordId are required"})
		}
		return s.svc.GetRecord(ctx, actor, model.ZoneID(in.ZoneID), model.RecordID(in.RecordID))
	})
	addTool(s, "dns_resolve", resolveDesc, false, true, func(ctx context.Context, actor auth.Actor, in resolveIn) (any, error) {
		ri, err := in.toResolve()
		if err != nil {
			return nil, domainerr.ValidationFailed("invalid client address",
				domainerr.FieldViolation{Path: "clientContext.client", Code: "invalid_value", Message: "client must be an IP address"})
		}
		out, err := s.svc.Resolve(ctx, actor, ri)
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": out.Result}, nil
	})
	addTool(s, "dns_explain_resolution", explainDesc, false, true, func(ctx context.Context, actor auth.Actor, in resolveIn) (any, error) {
		ri, err := in.toResolve()
		if err != nil {
			return nil, domainerr.ValidationFailed("invalid client address",
				domainerr.FieldViolation{Path: "clientContext.client", Code: "invalid_value", Message: "client must be an IP address"})
		}
		out, err := s.svc.Explain(ctx, actor, ri)
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": out.Result, "explanation": out.Explanation}, nil
	})
	addTool(s, "dns_forwarding_policies_list", fwdDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		pols, err := s.svc.ListForwardingPolicies(ctx, actor)
		if err != nil {
			return nil, err
		}
		if pols == nil {
			pols = []model.ForwardingPolicy{}
		}
		return map[string]any{"policies": pols}, nil
	})
	addTool(s, "dns_upstream_pools_list", poolsDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		pools, err := s.svc.ListUpstreamPools(ctx, actor)
		if err != nil {
			return nil, err
		}
		if pools == nil {
			pools = []model.UpstreamPool{}
		}
		return map[string]any{"pools": pools}, nil
	})
	addTool(s, "dns_upstreams_status", upsDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		ups, err := s.svc.UpstreamsStatus(ctx, actor)
		if err != nil {
			return nil, err
		}
		return map[string]any{"upstreams": fromUpstreams(ups)}, nil
	})
	addTool(s, "dns_cache_status", cacheStatusDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		st, err := s.svc.CacheStatus(ctx, actor)
		if err != nil {
			return nil, err
		}
		return fromCache(st), nil
	})
	addTool(s, "dns_cache_flush", cacheFlushDesc, true, true, func(ctx context.Context, actor auth.Actor, in flushIn) (any, error) {
		if err := s.svc.CacheFlush(ctx, actor, app.FlushIn{All: in.All}); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
	addTool(s, "dns_chaos_status", chaosStatusDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		st, err := s.svc.ChaosStatus(ctx, actor)
		if err != nil {
			return nil, err
		}
		return fromChaosStatus(st), nil
	})
	addTool(s, "dns_chaos_policies_list", chaosListDesc, false, true, func(ctx context.Context, actor auth.Actor, _ emptyIn) (any, error) {
		pols, err := s.svc.ListChaosPolicies(ctx, actor)
		if err != nil {
			return nil, err
		}
		if pols == nil {
			pols = []model.ChaosPolicy{}
		}
		return map[string]any{"policies": pols}, nil
	})
	addTool(s, "dns_chaos_policy_get", chaosGetDesc, false, true, func(ctx context.Context, actor auth.Actor, in idIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
		}
		return s.svc.GetChaosPolicy(ctx, actor, model.PolicyID(in.ID))
	})
	addTool(s, "dns_chaos_simulate", simulateDesc, false, true, func(ctx context.Context, actor auth.Actor, in simulateIn) (any, error) {
		si, err := in.toSimulate()
		if err != nil {
			return nil, domainerr.ValidationFailed("invalid client address",
				domainerr.FieldViolation{Path: "clientContext.client", Code: "invalid_value", Message: "client must be an IP address"})
		}
		out, err := s.svc.SimulateChaos(ctx, actor, si)
		if err != nil {
			return nil, err
		}
		return fromSimulate(out), nil
	})
	addTool(s, "dns_chaos_activate", activateDesc, true, false, func(ctx context.Context, actor auth.Actor, in activationIn) (any, error) {
		ain, err := in.toActivation()
		if err != nil {
			return nil, err
		}
		r, err := s.svc.ActivateChaos(ctx, actor, ain)
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "dns_chaos_deactivate", deactivateDesc, true, false, func(ctx context.Context, actor auth.Actor, in activationIn) (any, error) {
		ain, err := in.toActivation()
		if err != nil {
			return nil, err
		}
		r, err := s.svc.DeactivateChaos(ctx, actor, ain)
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "dns_chaos_set_expiry", expiryDesc, true, false, func(ctx context.Context, actor auth.Actor, in activationIn) (any, error) {
		ein, err := in.toExpiry()
		if err != nil {
			return nil, err
		}
		r, err := s.svc.SetChaosExpiry(ctx, actor, ein)
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "dns_chaos_emergency_disable", emergOffDesc, true, true, func(ctx context.Context, actor auth.Actor, in emergencyIn) (any, error) {
		r, err := s.svc.EmergencyDisableChaos(ctx, actor, app.EmergencyIn{Reason: in.Reason})
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "dns_chaos_emergency_enable", emergOnDesc, true, true, func(ctx context.Context, actor auth.Actor, in emergencyIn) (any, error) {
		r, err := s.svc.EmergencyEnableChaos(ctx, actor, app.EmergencyIn{Reason: in.Reason})
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "dns_audit_query", auditQueryDesc, false, true, func(ctx context.Context, actor auth.Actor, in auditQueryIn) (any, error) {
		list, err := s.svc.QueryAudit(ctx, actor, app.AuditQuery{Limit: in.Limit})
		if err != nil {
			return nil, err
		}
		return fromAuditList(list), nil
	})
	addTool(s, "dns_audit_get", auditGetDesc, false, true, func(ctx context.Context, actor auth.Actor, in idIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
		}
		ev, err := s.svc.GetAudit(ctx, actor, in.ID)
		if err != nil {
			return nil, err
		}
		return fromAuditEvent(ev), nil
	})
}

func addTool[In any](s *Server, name, desc string, mutating, idempotent bool, h func(context.Context, auth.Actor, In) (any, error)) {
	caps := capabilities.LookupTool(name)
	title := name
	if len(caps) > 0 && caps[0].Title != "" {
		title = caps[0].Title
		if desc == "" {
			desc = caps[0].Description
		}
	}
	readOnly := !mutating
	ann := &sdk.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    readOnly,
		IdempotentHint:  idempotent,
		DestructiveHint: boolPtr(mutating && !idempotent),
		OpenWorldHint:   boolPtr(false),
	}
	sdk.AddTool(s.sdk, &sdk.Tool{
		Name:        name,
		Title:       title,
		Description: desc,
		Annotations: ann,
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, any, error) {
		if err := ctx.Err(); err != nil {
			return toolErrorResult(canceledError(err)), nil, nil
		}
		actor := auth.LocalOrStdio(actorFrom(ctx))
		if err := s.authorizeTool(actor, name); err != nil {
			return toolErrorResult(err), nil, nil
		}
		out, err := h(ctx, actor, in)
		if err != nil {
			// Expected domain failures stay on the tool result so agents see
			// data.code without tearing down the Streamable HTTP request.
			return toolErrorResult(err), nil, nil
		}
		structured, err := asStructured(out)
		if err != nil {
			return nil, nil, rpcError(domainerr.Internal("internal error"))
		}
		return nil, structured, nil
	})
}

func boolPtr(v bool) *bool { return &v }

func canceledError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return domainerr.Internal("request deadline exceeded")
	}
	return domainerr.Internal("request canceled")
}

const (
	versionDesc     = "Read-only. Build and protocol versions (MCP " + ProtocolVersion + ")."
	capDesc         = "Read-only. Capability list and protocol metadata."
	statusDesc      = "Read-only. Agent-readable process status DTO."
	schemaDesc      = "Read-only. Published v1alpha1 config JSON Schema."
	docsDesc        = "Read-only. Embedded documentation. id is dns-semantics or chaos-safety."
	stateGetDesc    = "Read-only. Active revisions and a copy of canonical state."
	validateDesc    = "Read-only dry-run. Validate a candidate document and/or operations without writing."
	planDesc        = "Read-only dry-run. Plan operations against the active snapshot. Reversible: no swap."
	applyDesc       = "State-changing. Apply operations with expectedRevision and optional idempotencyKey. High-impact."
	exportDesc      = "Read-only. Canonical export plus bootstrap-to-runtime operations."
	resetDesc       = "State-changing, high-impact. Reread the bootstrap mount and swap. Not idempotent across drift. Never writes the file."
	zonesListDesc   = "Read-only. List configured zones."
	zoneGetDesc     = "Read-only. Get one zone by zoneId."
	recordsListDesc = "Read-only. List records in a zone."
	recordGetDesc   = "Read-only. Get one record by zoneId and recordId."
	resolveDesc     = "Read-only. Management-plane resolve. Defaults to not consuming live chaos."
	explainDesc     = "Read-only. Explain a resolve decision without changing live state."
	fwdDesc         = "Read-only. List compiled forwarding policies."
	poolsDesc       = "Read-only. List configured upstream pools."
	upsDesc         = "Read-only. Upstream health view."
	cacheStatusDesc = "Read-only. Process cache bounds and counters."
	cacheFlushDesc  = "State-changing. Flush the process cache. Does not change desired state. Idempotent."
	chaosStatusDesc = "Read-only. Chaos runtime status including the emergency-off bit."
	chaosListDesc   = "Read-only. List chaos policies."
	chaosGetDesc    = "Read-only. Get one chaos policy by id."
	simulateDesc    = "Read-only. Simulate chaos decisions without sleeping, sending packets, or mutating cache."
	activateDesc    = "State-changing. Activate a chaos policy via the shared apply path. High-impact."
	deactivateDesc  = "State-changing. Deactivate a chaos policy via the shared apply path."
	expiryDesc      = "State-changing. Extend or shorten a chaos policy expiry via the shared apply path."
	emergOffDesc    = "State-changing, high-impact. Set the runtime EmergencyChaosOff bit. Idempotent."
	emergOnDesc     = "State-changing, high-impact. Clear the runtime EmergencyChaosOff bit if YAML allows. Idempotent."
	auditQueryDesc  = "Read-only. Query recent in-memory audit events."
	auditGetDesc    = "Read-only. Get one audit event by id."
)
