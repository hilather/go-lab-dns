package testutil

import (
	"math/rand"
	"sync"
)

// Rand is an injectable 64-bit random source.
type Rand interface {
	Uint64() uint64
}

// SystemRand uses the process-wide math/rand source.
type SystemRand struct{}

// Uint64 returns a random uint64.
func (SystemRand) Uint64() uint64 { return rand.Uint64() }

// SeededRand is a mutex-protected deterministic RNG.
type SeededRand struct {
	mu sync.Mutex
	r  *rand.Rand
}

// NewSeededRand returns a Rand seeded with seed.
func NewSeededRand(seed int64) *SeededRand {
	return &SeededRand{r: rand.New(rand.NewSource(seed))}
}

// Uint64 returns the next deterministic uint64.
func (s *SeededRand) Uint64() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.r.Uint64()
}
