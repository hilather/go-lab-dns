package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/hilather/go-lab-dns/internal/buildinfo"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/domainerr"
)

// Version returns process buildinfo.
func (s *App) Version(ctx context.Context, actor Actor) (*buildinfo.Info, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	info := buildinfo.Current()
	return &info, nil
}

// Capabilities is a static first-GA name list. Bindings land in PR-09.
func (s *App) Capabilities(ctx context.Context, actor Actor) (*CapabilityView, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	return &CapabilityView{Capabilities: firstGACapabilities}, nil
}

func (s *App) Status(ctx context.Context, actor Actor) (*Status, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	info := buildinfo.Current()
	st := &Status{Version: info}
	if snap, err := s.active(); err == nil {
		st.Revisions = revisionView(snap)
		st.Listeners = []ListenerStatus{
			{Name: "dns", Address: snap.Listeners.DNSAddress},
			{Name: "management", Address: snap.Listeners.ManagementAddress},
		}
		if ch, err := s.ChaosStatus(ctx, actor); err == nil {
			st.Chaos = *ch
		}
		if ups, err := s.UpstreamsStatus(ctx, actor); err == nil {
			st.Upstreams = ups
		}
	}
	if cs, err := s.CacheStatus(ctx, actor); err == nil && cs != nil {
		st.Cache = *cs
	}
	return st, nil
}

func (s *App) ConfigSchema(ctx context.Context, actor Actor) ([]byte, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	b, err := config.SchemaBytes()
	if err != nil {
		return nil, domainerr.Internal("config schema: " + err.Error())
	}
	return append([]byte(nil), b...), nil
}

func (s *App) Docs(ctx context.Context, actor Actor, id string) ([]byte, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	rel, ok := docsRel[id]
	if !ok {
		return nil, domainerr.NotFound("docs " + id + " not found")
	}
	root, err := moduleRoot()
	if err != nil {
		return nil, domainerr.Internal("docs: " + err.Error())
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, domainerr.Internal("docs: " + err.Error())
	}
	return b, nil
}

var docsRel = map[string]string{
	"dns-semantics": "docs/02-dns-semantics.md",
	"chaos-safety":  "docs/03-chaos-engine.md",
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

var firstGACapabilities = []CapabilityInfo{
	{Name: "dns_version_get", Version: "v1", Description: "Build and protocol versions", Idempotent: true},
	{Name: "dns_capabilities_get", Version: "v1", Description: "Capability list", Idempotent: true},
	{Name: "dns_status_get", Version: "v1", Description: "Agent-readable status", Idempotent: true},
	{Name: "dns_schema_get", Version: "v1", Description: "Config JSON Schema", Idempotent: true},
	{Name: "dns_docs_get", Version: "v1", Description: "Embedded design docs", Idempotent: true},
	{Name: "dns_state_get", Version: "v1", Description: "Active revisions and canonical state", Idempotent: true},
	{Name: "dns_state_validate", Version: "v1", Description: "Validate a candidate", Mutating: false, Idempotent: true},
	{Name: "dns_change_plan", Version: "v1", Description: "Dry-run operations", Mutating: false, Idempotent: true},
	{Name: "dns_change_apply", Version: "v1", Description: "Apply operations", Mutating: true, Idempotent: true},
	{Name: "dns_state_export", Version: "v1", Description: "Canonical export and drift ops", Idempotent: true},
	{Name: "dns_state_reset", Version: "v1", Description: "Reread bootstrap and swap", Mutating: true, Idempotent: false},
	{Name: "dns_zones_list", Version: "v1", Description: "List zones", Idempotent: true},
	{Name: "dns_zone_get", Version: "v1", Description: "Get one zone", Idempotent: true},
	{Name: "dns_records_list", Version: "v1", Description: "List records", Idempotent: true},
	{Name: "dns_record_get", Version: "v1", Description: "Get one record", Idempotent: true},
	{Name: "dns_resolve", Version: "v1", Description: "Management resolve", Idempotent: true},
	{Name: "dns_explain_resolution", Version: "v1", Description: "Explain a resolve", Idempotent: true},
	{Name: "dns_forwarding_policies_list", Version: "v1", Description: "List forwarding policies", Idempotent: true},
	{Name: "dns_upstream_pools_list", Version: "v1", Description: "List upstream pools", Idempotent: true},
	{Name: "dns_upstreams_status", Version: "v1", Description: "Upstream health", Idempotent: true},
	{Name: "dns_cache_status", Version: "v1", Description: "Cache counters", Idempotent: true},
	{Name: "dns_cache_flush", Version: "v1", Description: "Flush process cache", Mutating: true, Idempotent: true},
	{Name: "dns_chaos_status", Version: "v1", Description: "Chaos runtime status", Idempotent: true},
	{Name: "dns_chaos_policies_list", Version: "v1", Description: "List chaos policies", Idempotent: true},
	{Name: "dns_chaos_policy_get", Version: "v1", Description: "Get one chaos policy", Idempotent: true},
	{Name: "dns_chaos_simulate", Version: "v1", Description: "Simulate chaos (CHA-001)"},
	{Name: "dns_chaos_activate", Version: "v1", Description: "Activate chaos (CHA-001)", Mutating: true},
	{Name: "dns_chaos_deactivate", Version: "v1", Description: "Deactivate chaos (CHA-001)", Mutating: true},
	{Name: "dns_chaos_set_expiry", Version: "v1", Description: "Set chaos expiry (CHA-001)", Mutating: true},
	{Name: "dns_chaos_emergency_disable", Version: "v1", Description: "Set EmergencyChaosOff", Mutating: true, Idempotent: true},
	{Name: "dns_chaos_emergency_enable", Version: "v1", Description: "Clear runtime EmergencyChaosOff", Mutating: true, Idempotent: true},
	{Name: "dns_audit_query", Version: "v1", Description: "List recent audit events", Idempotent: true},
	{Name: "dns_audit_get", Version: "v1", Description: "Get one audit event", Idempotent: true},
}
