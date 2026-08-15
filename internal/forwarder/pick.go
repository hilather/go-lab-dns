package forwarder

import (
	"sync"
	"sync/atomic"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// picker chooses the first upstream, then failover walks remaining
// configured order (skipping already tried). Health-aware prefers
// currently-healthy (or cooldown-expired) members; if none are eligible
// it last-resorts to configured order so a total outage still attempts.
type picker struct {
	mu     sync.Mutex
	rr     map[model.PoolID]*atomic.Uint64
	rand   testutil.Rand
	health *Health
}

func newPicker(rng testutil.Rand, h *Health) *picker {
	if rng == nil {
		rng = testutil.SystemRand{}
	}
	return &picker{rr: map[model.PoolID]*atomic.Uint64{}, rand: rng, health: h}
}

func (p *picker) order(pool *snapshot.CompiledPool) []snapshot.CompiledUpstream {
	if pool == nil || len(pool.Upstreams) == 0 {
		return nil
	}
	ups := append([]snapshot.CompiledUpstream(nil), pool.Upstreams...)
	start := p.startIndex(pool)
	if start <= 0 || start >= len(ups) {
		return p.preferHealthy(ups)
	}
	rotated := append(append([]snapshot.CompiledUpstream{}, ups[start:]...), ups[:start]...)
	return p.preferHealthy(rotated)
}

func (p *picker) startIndex(pool *snapshot.CompiledPool) int {
	n := len(pool.Upstreams)
	if n == 0 {
		return 0
	}
	switch pool.Strategy {
	case model.StrategyRoundRobin:
		return int(p.counter(pool.ID).Add(1)-1) % n
	case model.StrategyRandom:
		return int(p.rand.Uint64() % uint64(n))
	case model.StrategyOrdered, model.StrategyHealthAware:
		return 0
	default:
		return 0
	}
}

func (p *picker) preferHealthy(ups []snapshot.CompiledUpstream) []snapshot.CompiledUpstream {
	if p.health == nil || len(ups) < 2 {
		return ups
	}
	var healthy, rest []snapshot.CompiledUpstream
	for _, u := range ups {
		if p.health.Healthy(u.ID) {
			healthy = append(healthy, u)
		} else {
			rest = append(rest, u)
		}
	}
	if len(healthy) == 0 {
		return ups
	}
	return append(healthy, rest...)
}

func (p *picker) counter(id model.PoolID) *atomic.Uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.rr[id]
	if c == nil {
		c = &atomic.Uint64{}
		p.rr[id] = c
	}
	return c
}
