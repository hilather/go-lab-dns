package forwarder

import (
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// Health is query-driven only: no extra probe traffic. An upstream is marked
// down after DefaultFailThreshold consecutive timeout/transport failures and
// becomes probe-eligible after DefaultCooldown. SERVFAIL/REFUSED RCODEs do
// not change health (they are answers, not reachability).
const (
	DefaultFailThreshold = 2
	DefaultCooldown      = 30 * time.Second
)

// Health tracks per-upstream reachability. It is process-scoped runtime
// state, not a Snapshot field.
type Health struct {
	mu    sync.Mutex
	clk   testutil.Clock
	fails int
	cool  time.Duration
	byID  map[model.UpstreamID]*healthState
}

type healthState struct {
	fails    int
	down     bool
	downAt   time.Time
	lastFail time.Time
}

// NewHealth returns a tracker. clk nil uses the system clock.
func NewHealth(clk testutil.Clock) *Health {
	if clk == nil {
		clk = testutil.SystemClock{}
	}
	return &Health{
		clk:   clk,
		fails: DefaultFailThreshold,
		cool:  DefaultCooldown,
		byID:  map[model.UpstreamID]*healthState{},
	}
}

// Healthy reports whether id may be selected without being a last-resort pick.
func (h *Health) Healthy(id model.UpstreamID) bool {
	if h == nil || id == "" {
		return true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	st := h.byID[id]
	if st == nil || !st.down {
		return true
	}
	return h.clk.Now().Sub(st.downAt) >= h.cool
}

// RecordSuccess marks id healthy.
func (h *Health) RecordSuccess(id model.UpstreamID) {
	if h == nil || id == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.byID[id] = &healthState{}
}

// RecordFailure records a timeout or transport error.
func (h *Health) RecordFailure(id model.UpstreamID) {
	if h == nil || id == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	st := h.byID[id]
	if st == nil {
		st = &healthState{}
		h.byID[id] = st
	}
	st.fails++
	st.lastFail = h.clk.Now()
	if st.fails >= h.fails {
		st.down = true
		st.downAt = st.lastFail
	}
}

// Snapshot is a test/status view of one upstream.
func (h *Health) Snapshot(id model.UpstreamID) (down bool, fails int) {
	if h == nil {
		return false, 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	st := h.byID[id]
	if st == nil {
		return false, 0
	}
	return st.down, st.fails
}
