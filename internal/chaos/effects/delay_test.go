package effects

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestSleepFixedAndCancelReleasesBudget(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	eng := chaos.NewEngine(clk, nil)
	snap := delaySnap(1, 1)
	plan := chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionDelay, Phase: model.PhaseBeforeResponse, Delay: 50 * time.Millisecond, PolicyID: "p",
	}}}
	sess := NewSession(clk, eng.Budgets(), snap, eng.Stats())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sess.Sleep(ctx, plan, model.PhaseBeforeResponse) }()
	time.Sleep(20 * time.Millisecond)
	if eng.Budgets().InFlight() != 1 {
		t.Fatalf("inflight=%d", eng.Budgets().InFlight())
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("sleep did not return")
	}
	sess.Release()
	if eng.Budgets().InFlight() != 0 {
		t.Fatalf("leaked %d", eng.Budgets().InFlight())
	}
}

func TestSleepFiresOnAdvance(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	eng := chaos.NewEngine(clk, nil)
	snap := delaySnap(4, 4)
	plan := chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionDelay, Phase: model.PhaseBeforeResolution, Delay: 200 * time.Millisecond, PolicyID: "p",
	}}}
	sess := NewSession(clk, eng.Budgets(), snap, nil)
	defer sess.Release()
	done := make(chan error, 1)
	go func() { done <- sess.Sleep(context.Background(), plan, model.PhaseBeforeResolution) }()
	deadline := time.Now().Add(time.Second)
	for eng.Budgets().InFlight() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	clk.Advance(200 * time.Millisecond)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timer did not fire")
	}
}

func TestConcurrentDelayBudget(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	eng := chaos.NewEngine(clk, nil)
	snap := delaySnap(1, 1)
	plan := chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionDelay, Phase: model.PhaseBeforeResponse, Delay: time.Second, PolicyID: "p",
	}}}
	s1 := NewSession(clk, eng.Budgets(), snap, eng.Stats())
	s2 := NewSession(clk, eng.Budgets(), snap, eng.Stats())
	defer s1.Release()
	defer s2.Release()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	go func() {
		close(started)
		_ = s1.Sleep(ctx, plan, model.PhaseBeforeResponse)
	}()
	<-started
	time.Sleep(20 * time.Millisecond)
	if err := s2.Sleep(ctx, plan, model.PhaseBeforeResponse); err != nil {
		t.Fatal(err)
	}
	if !s2.BudgetSkipped() {
		t.Fatal("second delay must skip when maxConcurrentDelayed=1")
	}
}

func TestEmergencyCancelAllUnblocksDelays(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	eng := chaos.NewEngine(clk, nil)
	snap := delaySnap(64, 64)
	plan := chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionDelay, Phase: model.PhaseBeforeResponse, Delay: time.Hour, PolicyID: "p",
	}}}
	const n = 32
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := NewSession(clk, eng.Budgets(), snap, eng.Stats())
			defer s.Release()
			_ = s.Sleep(context.Background(), plan, model.PhaseBeforeResponse)
		}()
	}
	deadline := time.Now().Add(time.Second)
	for eng.Budgets().InFlight() < n && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if eng.Budgets().InFlight() != n {
		t.Fatalf("inflight=%d want %d", eng.Budgets().InFlight(), n)
	}
	eng.CancelDelays()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("emergency cancel did not unblock delays")
	}
}

func delaySnap(maxG, maxP int) *snapshot.Snapshot {
	p := model.ChaosPolicy{
		ID: "p", Enabled: true, SafetyClass: model.SafetyClassLow,
		Budget: &model.ChaosBudget{MaxConcurrency: maxP},
	}
	return &snapshot.Snapshot{
		Safety: snapshot.SafetyPolicy{MaxConcurrentDelayed: maxG, MaxDelay: 10 * time.Second},
		Chaos: snapshot.ChaosIndex{
			Enabled: true,
			ByID:    map[model.PolicyID]*snapshot.CompiledChaos{"p": {Policy: p}},
		},
	}
}
