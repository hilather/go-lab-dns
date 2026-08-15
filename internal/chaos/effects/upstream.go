package effects

import (
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
)

// Exchange maps the plan onto forwarder chaos hooks.
func Exchange(plan chaos.ActionPlan, metrics *chaos.Metrics) forwarder.ExchangeOpts {
	opts := forwarder.ExchangeOpts{
		ForceUpstream:       plan.Upstream.Force,
		ForceTimeout:        plan.Upstream.Timeout,
		ForceTransportError: plan.Upstream.TransportError,
		ForceFailover:       plan.Upstream.Failover,
		SyntheticRCode:      plan.Upstream.SyntheticRCode,
	}
	if len(plan.Upstream.Unavailable) > 0 {
		opts.Unavailable = map[model.UpstreamID]bool{}
		for _, id := range plan.Upstream.Unavailable {
			opts.Unavailable[id] = true
		}
	}
	if opts.ForceUpstream != "" || opts.ForceTimeout || opts.ForceTransportError || opts.ForceFailover || opts.SyntheticRCode != "" || len(opts.Unavailable) > 0 {
		if metrics != nil {
			metrics.UpstreamHooks.Add(1)
		}
	}
	return opts
}
