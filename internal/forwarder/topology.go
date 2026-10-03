package forwarder

import (
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// Observe each generation once. Shared state tracks the newest observed
// topology; old snapshot requests can finish without resurrecting retired IDs.
func (rt *Runtime) observeTopology(snap *snapshot.Snapshot) {
	rt.topologyMu.Lock()
	defer rt.topologyMu.Unlock()
	if rt.topologyObserved && snap.Generation <= rt.topologyGeneration {
		return
	}
	rt.topologyObserved = true
	rt.topologyGeneration = snap.Generation
	upstreams := make(map[model.UpstreamID]snapshot.CompiledUpstream)
	pools := make(map[model.PoolID]bool)
	for id, pool := range snap.Forwarding.Pools {
		if pool == nil {
			continue
		}
		if pool.Strategy == model.StrategyRoundRobin {
			pools[id] = true
		}
		for _, upstream := range pool.Upstreams {
			upstreams[upstream.ID] = upstream
		}
	}
	rt.Health.mu.Lock()
	for id := range rt.Health.byID {
		current, exists := upstreams[id]
		previous, known := rt.topologyUpstreams[id]
		if !exists || (known && current != previous) {
			delete(rt.Health.byID, id)
		}
	}
	rt.Health.mu.Unlock()
	rt.topologyUpstreams = upstreams
	rt.pick.mu.Lock()
	for id := range rt.pick.rr {
		if !pools[id] {
			delete(rt.pick.rr, id)
		}
	}
	rt.pick.mu.Unlock()
}

func (rt *Runtime) orderForSnapshot(snap *snapshot.Snapshot, pool *snapshot.CompiledPool) []snapshot.CompiledUpstream {
	rt.topologyMu.Lock()
	defer rt.topologyMu.Unlock()
	if snap.Generation < rt.topologyGeneration {
		// An old request still uses its configured pool. Its ephemeral counter
		// cannot restore removed pools or advance the current generation's state.
		return newPicker(rt.Rand, rt.Health).order(pool)
	}
	return rt.pick.order(pool)
}

func (rt *Runtime) recordHealth(snap *snapshot.Snapshot, id model.UpstreamID, success bool) {
	rt.topologyMu.Lock()
	defer rt.topologyMu.Unlock()
	if snap.Generation < rt.topologyGeneration {
		return
	}
	if success {
		rt.Health.RecordSuccess(id)
	} else {
		rt.Health.RecordFailure(id)
	}
}
