package chaos

import (
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

// Pressure is the process-scoped policy QPS / concurrency table.
// Simulation must never call Acquire.
type Pressure struct {
	mu        sync.Mutex
	by        map[model.PolicyID]*pressureState
	nextSweep time.Time
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
	if p.by == nil {
		p.by = map[model.PolicyID]*pressureState{}
	}
	cutoff := now.Add(-time.Second)
	if p.nextSweep.IsZero() || !now.Before(p.nextSweep) {
		for policy, state := range p.by {
			if state.inflight == 0 && (len(state.times) == 0 || !state.times[len(state.times)-1].After(cutoff)) {
				delete(p.by, policy)
			}
		}
		p.nextSweep = now.Add(time.Second)
	}
	st := p.by[id]
	if st == nil {
		st = &pressureState{}
		p.by[id] = st
	}
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
	if maxRate > 0 {
		st.times = append(st.times, now)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if st.inflight > 0 {
				st.inflight--
			}
			if st.inflight == 0 && len(st.times) == 0 {
				delete(p.by, id)
			}
		})
	}, false
}
