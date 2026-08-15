package mcp

import (
	"context"
	"strings"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	h := s.readResource
	s.sdk.AddResource(&sdk.Resource{
		URI: "labdns://state", Name: "state",
		Description: "Active revisions and canonical state (same as GET /v1/state).",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labdns://capabilities", Name: "capabilities",
		Description: "Capability list and protocol metadata.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labdns://status", Name: "status",
		Description: "Agent-readable process status DTO.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labdns://schema/config", Name: "schema-config",
		Description: "Published v1alpha1 config JSON Schema.",
		MIMEType:    "application/schema+json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labdns://upstreams", Name: "upstreams",
		Description: "Upstream health view.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labdns://audit/recent", Name: "audit-recent",
		Description: "Recent in-memory audit events.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labdns://docs/dns-semantics", Name: "docs-dns-semantics",
		Description: "Embedded DNS semantics document.",
		MIMEType:    "text/markdown",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labdns://docs/chaos-safety", Name: "docs-chaos-safety",
		Description: "Embedded chaos-engine safety document.",
		MIMEType:    "text/markdown",
	}, h)
	s.sdk.AddResourceTemplate(&sdk.ResourceTemplate{
		URITemplate: "labdns://zones/{zoneId}", Name: "zone",
		Description: "One zone by ID.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResourceTemplate(&sdk.ResourceTemplate{
		URITemplate: "labdns://records/{recordId}", Name: "record",
		Description: "One record by ID (scanned across zones).",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResourceTemplate(&sdk.ResourceTemplate{
		URITemplate: "labdns://chaos/policies/{policyId}", Name: "chaos-policy",
		Description: "One chaos policy by ID.",
		MIMEType:    "application/json",
	}, h)
}

func (s *Server) readResource(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, rpcError(domainerr.Internal("request canceled"))
	}
	actor := actorFrom(ctx)
	uri := ""
	if req != nil && req.Params != nil {
		uri = req.Params.URI
	}
	body, mime, err := s.resourceBody(ctx, actor, uri)
	if err != nil {
		return nil, rpcError(err)
	}
	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{{
			URI:      uri,
			MIMEType: mime,
			Text:     string(body),
		}},
	}, nil
}

func (s *Server) resourceBody(ctx context.Context, actor auth.Actor, uri string) ([]byte, string, error) {
	switch {
	case uri == "labdns://state":
		v, err := s.svc.GetState(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		view, err := fromStateView(v)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(view)
		return b, "application/json", err
	case uri == "labdns://capabilities":
		v, err := s.svc.Capabilities(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(fromCapabilities(v))
		return b, "application/json", err
	case uri == "labdns://status":
		v, err := s.svc.Status(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(fromStatus(v))
		return b, "application/json", err
	case uri == "labdns://schema/config":
		b, err := s.svc.ConfigSchema(ctx, actor)
		return b, "application/schema+json", err
	case uri == "labdns://upstreams":
		v, err := s.svc.UpstreamsStatus(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(map[string]any{"upstreams": fromUpstreams(v)})
		return b, "application/json", err
	case uri == "labdns://audit/recent":
		v, err := s.svc.QueryAudit(ctx, actor, app.AuditQuery{})
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(fromAuditList(v))
		return b, "application/json", err
	case uri == "labdns://docs/dns-semantics":
		b, err := s.svc.Docs(ctx, actor, "dns-semantics")
		return b, "text/markdown", err
	case uri == "labdns://docs/chaos-safety":
		b, err := s.svc.Docs(ctx, actor, "chaos-safety")
		return b, "text/markdown", err
	case strings.HasPrefix(uri, "labdns://zones/"):
		id := strings.TrimPrefix(uri, "labdns://zones/")
		if id == "" || strings.Contains(id, "/") {
			return nil, "", domainerr.NotFound("resource not found")
		}
		z, err := s.svc.GetZone(ctx, actor, model.ZoneID(id))
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(z)
		return b, "application/json", err
	case strings.HasPrefix(uri, "labdns://records/"):
		id := strings.TrimPrefix(uri, "labdns://records/")
		if id == "" || strings.Contains(id, "/") {
			return nil, "", domainerr.NotFound("resource not found")
		}
		rec, err := s.findRecord(ctx, actor, id)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(rec)
		return b, "application/json", err
	case strings.HasPrefix(uri, "labdns://chaos/policies/"):
		id := strings.TrimPrefix(uri, "labdns://chaos/policies/")
		if id == "" || strings.Contains(id, "/") {
			return nil, "", domainerr.NotFound("resource not found")
		}
		p, err := s.svc.GetChaosPolicy(ctx, actor, model.PolicyID(id))
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(p)
		return b, "application/json", err
	default:
		return nil, "", domainerr.NotFound("resource not found")
	}
}

func (s *Server) findRecord(ctx context.Context, actor auth.Actor, recordID string) (*model.Record, error) {
	// Catalog URI has no zoneId; scan every page and fail closed on ambiguity
	// or unexpected errors so a later zone-scoped deny is not swallowed.
	var found *model.Record
	page := app.Page{Limit: 256}
	for {
		zones, err := s.svc.ListZones(ctx, actor, page)
		if err != nil {
			return nil, err
		}
		if zones == nil {
			break
		}
		for _, z := range zones.Zones {
			rec, err := s.svc.GetRecord(ctx, actor, z.ID, model.RecordID(recordID))
			if err != nil {
				if de, ok := domainerr.As(err); ok && de.Code == domainerr.CodeNotFound {
					continue
				}
				return nil, err
			}
			if found != nil {
				return nil, domainerr.ValidationFailed("record id is not unique",
					domainerr.FieldViolation{Path: "recordId", Code: "ambiguous", Message: "record id matches multiple zones; use dns_record_get with zoneId"})
			}
			cp := *rec
			found = &cp
		}
		if zones.NextCursor == "" {
			break
		}
		page.Cursor = zones.NextCursor
	}
	if found == nil {
		return nil, domainerr.NotFound("record " + recordID + " not found")
	}
	return found, nil
}
