package perf

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/app"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestEmergencyDisableUnderLoad(t *testing.T) {
	st := LabState("")
	st.Spec.Chaos.Policies[1].Outcomes[0].Actions[0].Duration = 400 * time.Millisecond
	lab := NewLab(t, Options{State: st})
	svc := app.New(app.Options{Store: lab.Store, Engine: lab.Engine, Clock: lab.Clock})

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
	time.Sleep(20 * time.Millisecond)
	if _, err := svc.EmergencyDisableChaos(context.Background(), app.Actor{ID: "op", Class: "token"}, app.EmergencyIn{Reason: "load"}); err != nil {
		t.Fatal(err)
	}
	lab.Engine.CancelDelays()
	wg.Wait()

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
	_ = late
}
