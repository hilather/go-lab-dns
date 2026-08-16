package auth

import (
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
)

const (
	// DefaultMgmtRatePerSec is the per-source management QPS when unset.
	DefaultMgmtRatePerSec = 32
	// DefaultMgmtBurst is the per-source management burst when unset.
	DefaultMgmtBurst = 64
	// DefaultDNSQueryRatePerSec is the per-source DNS QPS when unset.
	DefaultDNSQueryRatePerSec = 256
	// DefaultDNSQueryBurst is the per-source DNS burst when unset.
	DefaultDNSQueryBurst = 512
)

// Limiter is a per-key token bucket. Negative rate disables limiting.
type Limiter struct {
	rate  float64
	burst float64
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*tbucket
}

type tbucket struct {
	tokens float64
	last   time.Time
}

// NewLimiter builds a limiter. Non-positive rate/burst use defaults when
// defaultRate/defaultBurst are positive; rate < 0 means unlimited.
func NewLimiter(rate, burst float64, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		rate:    rate,
		burst:   burst,
		now:     now,
		buckets: map[string]*tbucket{},
	}
}

// ManagementLimiter is the first-GA management default.
func ManagementLimiter(rate, burst float64, now func() time.Time) *Limiter {
	if rate < 0 {
		return NewLimiter(-1, 0, now)
	}
	if rate == 0 {
		rate = DefaultMgmtRatePerSec
	}
	if burst <= 0 {
		burst = DefaultMgmtBurst
	}
	return NewLimiter(rate, burst, now)
}

// DNSLimiter is the first-GA DNS query default.
func DNSLimiter(rate, burst float64, now func() time.Time) *Limiter {
	if rate < 0 {
		return NewLimiter(-1, 0, now)
	}
	if rate == 0 {
		rate = DefaultDNSQueryRatePerSec
	}
	if burst <= 0 {
		burst = DefaultDNSQueryBurst
	}
	return NewLimiter(rate, burst, now)
}

// Allow reports whether key may proceed and consumes one token.
func (l *Limiter) Allow(key string) bool {
	if l == nil || l.rate < 0 {
		return true
	}
	if l.rate == 0 {
		return false
	}
	if key == "" {
		key = "unknown"
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		b = &tbucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// AllowErr is Allow as a domain error.
func (l *Limiter) AllowErr(key string) error {
	if l.Allow(key) {
		return nil
	}
	return domainerr.RateLimited("rate limit exceeded")
}
