package chaos

import (
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// Pressure is the process-scoped policy QPS / concurrency table.
// Simulation must never call Acquire.
type Pressure struct {
	mu sync.Mutex
	by map[model.PolicyID]*pressureState
}

type pressureState struct {
	inflight int
	times    []time.Time
}

// NewPressure returns an empty tracker.
func NewPressure() *Pressure {
	return &Pressure{by: map[model.PolicyID]*pressureState{}}
}

// Acquire accounts one matching query. exceeded is true when the
// configured QPS or concurrency cap is already at the limit.
// A zero cap is unlimited for that dimension. release is always non-nil.
func (p *Pressure) Acquire(id model.PolicyID, maxConc int, maxRate float64, now time.Time) (release func(), exceeded bool) {
	noop := func() {}
	if p == nil || id == "" {
		return noop, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	st := p.by[id]
	if st == nil {
		st = &pressureState{}
		p.by[id] = st
	}
	cutoff := now.Add(-time.Second)
	kept := st.times[:0]
	for _, t := range st.times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	st.times = kept

	if maxConc > 0 && st.inflight >= maxConc {
		exceeded = true
	}
	if maxRate > 0 && float64(len(st.times)) >= maxRate {
		exceeded = true
	}
	if exceeded {
		return noop, true
	}
	st.inflight++
	st.times = append(st.times, now)
	var once sync.Once
	return func() {
		once.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if st.inflight > 0 {
				st.inflight--
			}
		})
	}, false
}
