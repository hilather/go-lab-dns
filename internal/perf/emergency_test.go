package perf

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestEmergencyDisableUnderLoad(t *testing.T) {
	st := LabState("")
	st.Spec.Chaos.Policies[1].Outcomes[0].Actions[0].Duration = 400 * time.Millisecond
	lab := NewLab(t, Options{State: st})

	var wg sync.WaitGroup
	var late atomic.Int64
	start := time.Now()
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, _, err := lab.ServeHint(context.Background(), QueryDelay())
			if err != nil {
				return
			}
			if res != nil && res.Result().RCode != model.RCodeNoError {
				t.Errorf("rcode=%s", res.Result().RCode)
			}
			if time.Since(start) > 250*time.Millisecond {
				late.Add(1)
			}
		}()
	}
	deadline := time.Now().Add(100 * time.Millisecond)
	for lab.Engine.Budgets().InFlight() < 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	// Production combined path (SIGUSR1 and app.EmergencyDisableChaos):
	// inhibit bit + cancel in-flight delays.
	chaos.EmergencyDisable(lab.Store, lab.Engine)
	wg.Wait()
	if n := late.Load(); n != 0 {
		t.Fatalf("in-flight delays not cancelled: late=%d (elapsed > 250ms)", n)
	}

	// After inhibit, a new query must not take the 400ms delay.
	t0 := time.Now()
	res := lab.Serve(t, QueryDelay())
	if res.RCode != model.RCodeNoError {
		t.Fatalf("rcode=%s", res.RCode)
	}
	if time.Since(t0) > 150*time.Millisecond {
		t.Fatalf("emergency disable still delayed: %s", time.Since(t0))
	}
	if lab.Engine.Budgets().InFlight() != 0 {
		t.Fatalf("reservations after emergency: %d", lab.Engine.Budgets().InFlight())
	}
}
