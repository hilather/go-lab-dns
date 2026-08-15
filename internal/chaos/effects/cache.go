package effects

import (
	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/chaos"
)

// CacheGet maps the plan onto a cache lookup hook. Bypass wins over
// the other flags so a composed policy cannot both skip and mutate.
func CacheGet(plan chaos.ActionPlan, metrics *chaos.Metrics) cache.GetOpts {
	opts := cache.GetOpts{
		Bypass:       plan.Cache.Bypass,
		ForceMiss:    plan.Cache.ForceMiss,
		ServeStale:   plan.Cache.ServeStale,
		TreatExpired: plan.Cache.Expire,
	}
	if opts.Bypass || opts.ForceMiss || opts.ServeStale || opts.TreatExpired {
		if metrics != nil {
			metrics.CacheHooks.Add(1)
		}
	}
	return opts
}

// CachePut maps skip-store hooks. Bypass/force-miss do not skip put
// so the next request can still observe the real entry.
func CachePut(plan chaos.ActionPlan) cache.PutOpts {
	return cache.PutOpts{Skip: plan.Cache.Bypass}
}
