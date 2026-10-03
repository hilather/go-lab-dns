package chaos

import (
	"fmt"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestDelayBudgetDoesNotRetainReleasedPolicyIDs(t *testing.T) {
	b := NewBudgets()
	for i := 0; i < 100; i++ {
		tok, err := b.ReserveDelay(model.PolicyID(fmt.Sprint(i)), 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		tok.Release()
	}
	if len(b.perPolicy) != 0 {
		t.Fatalf("retained %d released policy IDs", len(b.perPolicy))
	}
}

func TestPressureDoesNotRetainUnlimitedRateSamples(t *testing.T) {
	p := NewPressure()
	now := time.Unix(100, 0)
	for i := 0; i < 100; i++ {
		release, exceeded := p.Acquire("p", 1, 0, now)
		if exceeded {
			t.Fatal("unexpected rejection")
		}
		release()
	}
	if len(p.by) != 0 {
		t.Fatalf("retained %d idle unlimited-rate policies", len(p.by))
	}
}

func TestPressurePrunesExpiredIdlePolicies(t *testing.T) {
	p := NewPressure()
	now := time.Unix(100, 0)
	for i := 0; i < 100; i++ {
		release, exceeded := p.Acquire(model.PolicyID(fmt.Sprint(i)), 1, 1, now)
		if exceeded {
			t.Fatal("unexpected rejection")
		}
		release()
	}
	release, exceeded := p.Acquire("active", 1, 1, now.Add(2*time.Second))
	if exceeded {
		t.Fatal("unexpected rejection")
	}
	defer release()
	if len(p.by) != 1 {
		t.Fatalf("retained %d entries after rate windows expired, want 1", len(p.by))
	}
}

func TestUnregisterCancelClearsRetainedClosure(t *testing.T) {
	b := NewBudgets()
	remove := b.WatchCancel(func() {})
	b.WatchCancel(func() {})
	remove()
	if len(b.cancels) != 1 {
		t.Fatal("remaining watch lost")
	}
	if b.cancels[:cap(b.cancels)][1].fn != nil {
		t.Fatal("unregistered callback retained in backing array")
	}
}
