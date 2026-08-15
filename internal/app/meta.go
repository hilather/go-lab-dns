package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/hilather/go-lab-dns/internal/buildinfo"
	"github.com/hilather/go-lab-dns/internal/capabilities"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/observability"
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

// Capabilities is the frozen registry discovery list.
func (s *App) Capabilities(ctx context.Context, actor Actor) (*CapabilityView, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	src := capabilities.DiscoveryList()
	out := make([]CapabilityInfo, len(src))
	for i, d := range src {
		out[i] = CapabilityInfo{
			Name:        d.Name,
			Version:     d.Version,
			Description: d.Description,
			Mutating:    d.Mutating,
			Idempotent:  d.Idempotent,
		}
	}
	return &CapabilityView{Capabilities: out}, nil
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
	s.annotateStatus(st)
	return st, nil
}

func (s *App) annotateStatus(st *Status) {
	if st == nil {
		return
	}
	unhealthy := 0
	for _, u := range st.Upstreams {
		if !u.Healthy {
			unhealthy++
		}
	}
	facts := observability.Facts{
		HasRuntimeRevision: st.Revisions.RuntimeRevision != "",
		Drifted:            st.Revisions.Drifted,
		EmergencyChaos:     st.Chaos.EmergencyDisabled,
		ChaosEnabled:       st.Chaos.Enabled,
		UnhealthyUpstreams: unhealthy,
		CacheEntries:       st.Cache.Entries,
		CacheMax:           st.Cache.MaxEntries,
	}
	if s != nil && s.healthSrc != nil {
		facts.ProcessDown = s.healthSrc.ProcessDown()
		facts.DNSDown = s.healthSrc.DNSDown()
		facts.MgmtDown = s.healthSrc.MgmtDown()
		facts.TelemetryDrops = s.healthSrc.TelemetryDrops()
	}
	if s != nil && s.metrics != nil {
		facts.TelemetryDrops += s.metrics.Dropped()
		gen := float64(st.Revisions.Generation)
		s.metrics.Set(observability.MetricStateGeneration, nil, gen)
		drift := 0.0
		if st.Revisions.Drifted {
			drift = 1
		}
		s.metrics.Set(observability.MetricStateDrifted, nil, drift)
		em := 0.0
		if st.Chaos.EmergencyDisabled {
			em = 1
		}
		s.metrics.Set(observability.MetricChaosEmergency, nil, em)
	}
	probe := observability.Evaluate(facts)
	st.Ready = probe.Ready
	st.Degraded = probe.Degraded
	if len(probe.Warnings) > 0 {
		st.Warnings = make([]Warning, len(probe.Warnings))
		for i, w := range probe.Warnings {
			st.Warnings[i] = Warning{Code: w.Code, Message: w.Message}
		}
	}
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
