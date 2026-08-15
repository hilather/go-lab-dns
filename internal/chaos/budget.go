package chaos

import (
	"sync"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

// Budgets is the process-scoped delayed-request reservation table.
// Simulation must never call Reserve.
type Budgets struct {
	mu        sync.Mutex
	global    int
	perPolicy map[model.PolicyID]int
	cancels   []cancelReg
	nextID    uint64
}

type cancelReg struct {
	id uint64
	fn func()
}

// NewBudgets returns an empty reservation table.
func NewBudgets() *Budgets {
	return &Budgets{perPolicy: map[model.PolicyID]int{}}
}

// Token releases one reservation. Release is idempotent.
type Token struct {
	b     *Budgets
	id    model.PolicyID
	once  sync.Once
	valid bool
}

// ReserveDelay accounts one in-flight delay against global and per-policy
// caps. A zero/negative cap is unlimited for that dimension.
func (b *Budgets) ReserveDelay(id model.PolicyID, maxGlobal, maxPolicy int) (*Token, error) {
	if b == nil {
		return &Token{}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if maxGlobal > 0 && b.global >= maxGlobal {
		return nil, domainerr.ChaosBudgetExceeded("maxConcurrentDelayed exhausted")
	}
	if maxPolicy > 0 && b.perPolicy[id] >= maxPolicy {
		return nil, domainerr.ChaosBudgetExceeded("policy maxConcurrency exhausted")
	}
	b.global++
	if b.perPolicy == nil {
		b.perPolicy = map[model.PolicyID]int{}
	}
	b.perPolicy[id]++
	return &Token{b: b, id: id, valid: true}, nil
}

// Release decrements the reservation. Safe on a nil token.
func (t *Token) Release() {
	if t == nil || !t.valid || t.b == nil {
		return
	}
	t.once.Do(func() {
		t.b.mu.Lock()
		defer t.b.mu.Unlock()
		if t.b.global > 0 {
			t.b.global--
		}
		if t.b.perPolicy != nil && t.b.perPolicy[t.id] > 0 {
			t.b.perPolicy[t.id]--
		}
	})
}

// InFlight is the current global delayed-request count.
func (b *Budgets) InFlight() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.global
}

// PolicyInFlight is the current per-policy delayed-request count.
func (b *Budgets) PolicyInFlight(id model.PolicyID) int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.perPolicy[id]
}

// RegisterCancel records a function invoked by CancelAll (emergency).
func (b *Budgets) RegisterCancel(fn func()) {
	_ = b.WatchCancel(fn)
}

// WatchCancel records a cancel func and returns an unregister that is
// safe to call more than once. Used so finished delays do not leak.
func (b *Budgets) WatchCancel(fn func()) (unregister func()) {
	if b == nil || fn == nil {
		return func() {}
	}
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.cancels = append(b.cancels, cancelReg{id: id, fn: fn})
	b.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			kept := b.cancels[:0]
			for _, c := range b.cancels {
				if c.id != id {
					kept = append(kept, c)
				}
			}
			b.cancels = kept
		})
	}
}

// CancelAll runs registered cancel funcs (outstanding delays) and leaves
// reservations for those funcs to Release.
func (b *Budgets) CancelAll() {
	if b == nil {
		return
	}
	b.mu.Lock()
	fns := make([]func(), 0, len(b.cancels))
	for _, c := range b.cancels {
		if c.fn != nil {
			fns = append(fns, c.fn)
		}
	}
	b.cancels = nil
	b.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

func clampDelay(d, globalMax, policyMax time.Duration) (time.Duration, bool) {
	out := d
	if policyMax > 0 && out > policyMax {
		out = policyMax
	}
	if globalMax > 0 && out > globalMax {
		out = globalMax
	}
	return out, out != d
}
