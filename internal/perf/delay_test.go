package perf

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
)

func TestMaxDelayedConcurrency(t *testing.T) {
	const capN = 4
	const delay = 80 * time.Millisecond
	const extra = 12

	st := LabState("")
	st.Spec.Chaos.Safety.MaxConcurrentDelayed = capN
	st.Spec.Chaos.Policies[1].Outcomes[0].Actions[0].Duration = delay
	lab := NewLab(t, Options{State: st})

	var peak atomic.Int64
	var wg sync.WaitGroup
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				n := int64(lab.Engine.Budgets().InFlight())
				for {
					cur := peak.Load()
					if n <= cur || peak.CompareAndSwap(cur, n) {
						break
					}
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()

	start := time.Now()
	for i := 0; i < capN+extra; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, hint, err := lab.ServeHint(context.Background(), QueryDelay())
			if err != nil {
				t.Errorf("serve: %v", err)
				return
			}
			if hint != 0 && res == nil {
				t.Errorf("empty response hint=%v", hint)
			}
			if res != nil && res.Result().RCode != model.RCodeNoError {
				t.Errorf("rcode=%s", res.Result().RCode)
			}
		}()
	}
	wg.Wait()
	close(stop)
	elapsed := time.Since(start)

	if p := peak.Load(); p > capN {
		t.Fatalf("peak delayed inflight=%d exceeds cap %d", p, capN)
	}
	if lab.Engine.Budgets().InFlight() != 0 {
		t.Fatalf("leaked reservations: %d", lab.Engine.Budgets().InFlight())
	}
	if skipped := lab.Engine.Stats().BudgetSkipped.Load(); skipped < 1 {
		t.Fatalf("expected budget skips above the cap, skipped=%d", skipped)
	}
	// Everyone answers; extras skip sleep so wall time stays near one delay.
	if elapsed > 3*delay {
		t.Fatalf("elapsed %s, want near one delay (cap=%d)", elapsed, capN)
	}
}

func TestDelayBudgetReleasedAfterCancel(t *testing.T) {
	st := LabState("")
	st.Spec.Chaos.Safety.MaxConcurrentDelayed = 8
	st.Spec.Chaos.Policies[1].Outcomes[0].Actions[0].Duration = 200 * time.Millisecond
	lab := NewLab(t, Options{State: st})

	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = lab.ServeHint(ctx, QueryDelay())
		}()
	}
	time.Sleep(20 * time.Millisecond)
	cancel()
	lab.Engine.CancelDelays()
	wg.Wait()
	if lab.Engine.Budgets().InFlight() != 0 {
		t.Fatalf("reservations after cancel: %d", lab.Engine.Budgets().InFlight())
	}
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+16 {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
}
