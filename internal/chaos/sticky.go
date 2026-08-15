package chaos

import (
	"sync"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// StickyRand holds one live query's random-mode draws so pre- and
// post-resolution Decide pick the same outcome without writing hash-v1
// field 10 (that field stays empty unless Simulate).
type StickyRand struct {
	mu sync.Mutex
	by map[model.PolicyID]HashResult
}

// NewStickyRand returns an empty per-query draw table.
func NewStickyRand() *StickyRand {
	return &StickyRand{by: map[model.PolicyID]HashResult{}}
}

// Draw returns the first random uniforms for id, generating them once.
func (s *StickyRand) Draw(id model.PolicyID, rng testutil.Rand) HashResult {
	if rng == nil {
		rng = testutil.SystemRand{}
	}
	if s == nil {
		return randomDraw(rng)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.by == nil {
		s.by = map[model.PolicyID]HashResult{}
	}
	if h, ok := s.by[id]; ok {
		return h
	}
	h := randomDraw(rng)
	s.by[id] = h
	return h
}

func randomDraw(rng testutil.Rand) HashResult {
	p := unit(rng.Uint64())
	w := unit(rng.Uint64())
	return HashResult{P: p, W: w, U1: rng.Uint64()}
}
