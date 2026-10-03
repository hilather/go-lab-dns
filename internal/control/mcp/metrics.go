package mcp

import (
	"github.com/hilather/go-lab-dns/internal/observability"
	"time"
)

func (s *Server) observeCapability(capability string, started time.Time, failed bool) {
	if s.cfg.Metrics == nil {
		return
	}
	result := "ok"
	if failed {
		result = "error"
	}
	s.cfg.Metrics.Inc(observability.MetricCapabilityCalls, map[string]string{"capability": capability, "transport": "mcp", "result": result}, 1)
	s.cfg.Metrics.Observe(observability.MetricCapabilityDuration, map[string]string{"capability": capability, "transport": "mcp"}, time.Since(started).Seconds())
}
