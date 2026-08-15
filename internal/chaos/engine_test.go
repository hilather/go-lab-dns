package chaos

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func TestDecideRecordAndGlobal(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	eng := NewEngine(clk, testutil.NewSeededRand(1))
	snap := mustSnap(t)
	q := model.Query{Name: "foo.tools.lab.example.net.", Type: model.TypeA, Class: model.ClassIN, Transport: model.TransportUDP, Client: netip.MustParseAddr("10.42.0.10")}
	wild := model.RecordID("tools-wildcard-a")
	plan, err := eng.Decide(context.Background(), snap, DecisionIn{
		Query: q, ClientGroupID: "test-devices", ZoneID: "lab-zone",
		Base:  &model.Result{RCode: model.RCodeNoError, WildcardSource: &wild, Answers: []model.RR{{Name: q.Name, Type: model.TypeA}}},
		Phase: PhaseResponse,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Disabled {
		t.Fatalf("disabled: %s", plan.Reason)
	}
	var triggered []string
	for _, d := range plan.Decisions {
		if d.Triggered {
			triggered = append(triggered, string(d.PolicyID))
		}
	}
	if len(triggered) == 0 {
		t.Fatalf("no trigger: %+v", plan.Decisions)
	}
	found := false
	for _, id := range triggered {
		if id == "slow-tools" {
			found = true
		}
	}
	if !found {
		t.Fatalf("slow-tools not selected: %v", triggered)
	}
}

func TestEmergencyDisablesDecide(t *testing.T) {
	eng := NewEngine(testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)), nil)
	snap := mustSnap(t)
	snap.EmergencyChaosOff = true
	plan, err := eng.Decide(context.Background(), snap, DecisionIn{
		Query:         model.Query{Name: "foo.tools.lab.example.net.", Type: model.TypeA, Transport: model.TransportUDP},
		ClientGroupID: "test-devices", Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Disabled || plan.Reason != "emergency_disabled" {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestProtectedNameAndExemptGroup(t *testing.T) {
	eng := NewEngine(testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)), nil)
	snap := mustSnap(t)
	plan, err := eng.Decide(context.Background(), snap, DecisionIn{
		Query:         model.Query{Name: "dns.lab.example.net.", Type: model.TypeA, Transport: model.TransportUDP},
		ClientGroupID: "test-devices", Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Reason != "protected_name" {
		t.Fatalf("reason=%s", plan.Reason)
	}
	plan, err = eng.Decide(context.Background(), snap, DecisionIn{
		Query:         model.Query{Name: "foo.tools.lab.example.net.", Type: model.TypeA, Transport: model.TransportUDP},
		ClientGroupID: "management", Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Reason != "protected_client_group" && plan.Reason != "chaos_exempt" {
		t.Fatalf("reason=%s", plan.Reason)
	}
}

func TestStartExpiryFlapEveryNth(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	eng := NewEngine(clk, nil)
	st := sampleState(t)
	start := clk.Now().Add(time.Hour)
	st.Spec.Chaos.Policies[1].StartsAt = &start
	st.Spec.Chaos.Policies[1].Selector.EveryNth = 0
	snap := compileSnap(t, st)

	plan, err := eng.Decide(context.Background(), snap, DecisionIn{
		Query: model.Query{Name: "x.example.", Type: model.TypeA, Transport: model.TransportUDP},
		Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasSkip(plan, "global-delay", "not_started") {
		t.Fatalf("expected not_started: %+v", plan.Decisions)
	}

	clk.Advance(2 * time.Hour)
	exp := clk.Now().Add(-time.Second)
	st.Spec.Chaos.Policies[1].StartsAt = nil
	st.Spec.Chaos.Policies[1].ExpiresAt = &exp
	snap = compileSnap(t, st)
	plan, err = eng.Decide(context.Background(), snap, DecisionIn{
		Query: model.Query{Name: "x.example.", Type: model.TypeA, Transport: model.TransportUDP},
		Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasSkip(plan, "global-delay", "expired") {
		t.Fatalf("expected expired: %+v", plan.Decisions)
	}

	st = sampleState(t)
	st.Spec.Chaos.Policies[1].Selector.Period = 10 * time.Second
	st.Spec.Chaos.Policies[1].Selector.Unhealthy = 2 * time.Second
	st.Spec.Chaos.Policies[1].Selector.PhaseOffset = 0
	// now=20:00:00 → elapsed 0? Unix of 2026-08-15T20:00:00 is large;  % 10s
	snap = compileSnap(t, st)
	clk2 := testutil.NewFakeClock(time.Unix(1_000_000_003, 0).UTC()) // 3s into a 10s period if offset 0 from epoch
	eng2 := NewEngine(clk2, nil)
	plan, err = eng2.Decide(context.Background(), snap, DecisionIn{
		Query: model.Query{Name: "x.example.", Type: model.TypeA, Transport: model.TransportUDP},
		Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasSkip(plan, "global-delay", "flap_healthy") {
		t.Fatalf("expected flap_healthy at +3s: %+v", plan.Decisions)
	}

	st.Spec.Chaos.Policies[1].Selector.Period = 0
	st.Spec.Chaos.Policies[1].Selector.Unhealthy = 0
	st.Spec.Chaos.Policies[1].Selector.EveryNth = 3
	st.Spec.Chaos.Policies[1].Selector.TimeBucket = time.Second
	snap = compileSnap(t, st)
	// unix 1_000_000_001 % 3 == 1 → skip
	eng3 := NewEngine(testutil.NewFakeClock(time.Unix(1_000_000_001, 0).UTC()), nil)
	plan, err = eng3.Decide(context.Background(), snap, DecisionIn{
		Query: model.Query{Name: "x.example.", Type: model.TypeA, Transport: model.TransportUDP},
		Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasSkip(plan, "global-delay", "every_nth") {
		t.Fatalf("expected every_nth: %+v", plan.Decisions)
	}
}

func TestCompositionTerminalAndExclusive(t *testing.T) {
	st := sampleState(t)
	st.Spec.Chaos.Policies[1].Composition = model.CompositionTerminal
	// Add another global that would run after if compose.
	st.Spec.Chaos.Policies = append(st.Spec.Chaos.Policies, model.ChaosPolicy{
		ID: "after-terminal", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "z", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "x", Weight: 1, Actions: []model.ChaosAction{{Type: model.ActionDelay, Phase: model.PhaseBeforeResolution, Duration: time.Millisecond}}}},
	})
	snap := compileSnap(t, st)
	eng := NewEngine(testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)), nil)
	plan, err := eng.Decide(context.Background(), snap, DecisionIn{
		Query: model.Query{Name: "x.example.", Type: model.TypeA, Transport: model.TransportUDP},
		Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range plan.Decisions {
		if d.PolicyID == "after-terminal" && d.Triggered {
			t.Fatal("terminal did not stop evaluation")
		}
	}

	st = sampleState(t)
	st.Spec.Chaos.Policies[1].Composition = model.CompositionExclusiveGroup
	st.Spec.Chaos.Policies[1].ExclusiveGroup = "g1"
	st.Spec.Chaos.Policies = append(st.Spec.Chaos.Policies, model.ChaosPolicy{
		ID: "also-g1", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Composition: model.CompositionExclusiveGroup, ExclusiveGroup: "g1",
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "z", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "x", Weight: 1, Actions: []model.ChaosAction{{Type: model.ActionDelay, Phase: model.PhaseBeforeResolution, Duration: time.Millisecond}}}},
	})
	snap = compileSnap(t, st)
	plan, err = eng.Decide(context.Background(), snap, DecisionIn{
		Query: model.Query{Name: "x.example.", Type: model.TypeA, Transport: model.TransportUDP},
		Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasSkip(plan, "also-g1", "exclusive_group:global-delay") {
		t.Fatalf("exclusive: %+v", plan.Decisions)
	}
}

func TestDelayClampAndSimulateNoBudget(t *testing.T) {
	st := sampleState(t)
	st.Spec.Chaos.Safety.MaxDelay = 10 * time.Millisecond
	st.Spec.Chaos.Policies[1].Outcomes[0].Actions[0].Duration = time.Second
	snap := compileSnap(t, st)
	eng := NewEngine(testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)), nil)
	before := eng.Budgets().InFlight()
	out, err := eng.Simulate(context.Background(), snap, SimulateIn{
		Query: model.Query{Name: "x.example.", Type: model.TypeA, Transport: model.TransportUDP},
		Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.BudgetUsed {
		t.Fatal("simulate consumed budget")
	}
	if eng.Budgets().InFlight() != before {
		t.Fatal("simulate changed in-flight")
	}
	if !out.Triggered {
		t.Fatalf("expected trigger: %+v", out.Decisions)
	}
	clamped := false
	for _, c := range out.Plan.Clamped {
		if c.Reason == "max_delay" {
			clamped = true
		}
	}
	if !clamped {
		t.Fatalf("expected clamp: %+v", out.Plan.Clamped)
	}
}

func TestSimulateDoesNotSleep(t *testing.T) {
	clk := testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	eng := NewEngine(clk, nil)
	snap := mustSnap(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = eng.Simulate(context.Background(), snap, SimulateIn{
			Query:         model.Query{Name: "foo.tools.lab.example.net.", Type: model.TypeA, Transport: model.TransportUDP},
			ClientGroupID: "test-devices", ZoneID: "lab-zone", Phase: PhasePreResolution, Nonce: "n1",
		})
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("simulate blocked (slept?)")
	}
}

func TestWeightedRandomStatisticalBounds(t *testing.T) {
	st := sampleState(t)
	st.Spec.Zones[0].Records[0].ChaosPolicyRefs = nil
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID: "rand", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Selector: model.ChaosSelector{Mode: model.SelectorRandom, Probability: 1},
		Outcomes: []model.ChaosOutcome{
			{ID: "a", Weight: 1, Actions: []model.ChaosAction{{Type: model.ActionDelay, Phase: model.PhaseBeforeResolution, Duration: time.Millisecond}}},
			{ID: "b", Weight: 1, Actions: []model.ChaosAction{{Type: model.ActionDelay, Phase: model.PhaseBeforeResolution, Duration: time.Millisecond}}},
		},
	}}
	snap := compileSnap(t, st)
	eng := NewEngine(testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)), testutil.NewSeededRand(42))
	n := 4000
	countA := 0
	for i := 0; i < n; i++ {
		plan, err := eng.Decide(context.Background(), snap, DecisionIn{
			Query: model.Query{Name: "x.example.", Type: model.TypeA, Transport: model.TransportUDP},
			Phase: PhasePreResolution,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range plan.Decisions {
			if d.Triggered && d.OutcomeID == "a" {
				countA++
			}
		}
	}
	// 50/50; 4-sigma on binomial ≈ 126. Fail only on gross bias.
	if countA < 1700 || countA > 2300 {
		t.Fatalf("countA=%d / %d", countA, n)
	}
}

func TestDecideCanceledContext(t *testing.T) {
	eng := NewEngine(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := eng.Decide(ctx, mustSnap(t), DecisionIn{})
	if err == nil {
		t.Fatal("expected cancel")
	}
}

func TestConcurrentBudgetReserveRelease(t *testing.T) {
	b := NewBudgets()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, err := b.ReserveDelay("p", 100, 100)
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			tok.Release()
			tok.Release()
		}()
	}
	wg.Wait()
	if b.InFlight() != 0 {
		t.Fatalf("leaked %d", b.InFlight())
	}
}

func TestBudgetExhausted(t *testing.T) {
	b := NewBudgets()
	tok, err := b.ReserveDelay("p", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReserveDelay("p", 1, 1); err == nil {
		t.Fatal("expected exhausted")
	}
	tok.Release()
	if _, err := b.ReserveDelay("p", 1, 1); err != nil {
		t.Fatal(err)
	}
}

func TestPlanSummaryCacheUpstreamPressure(t *testing.T) {
	st := sampleState(t)
	for i := range st.Spec.Zones[0].Records {
		st.Spec.Zones[0].Records[i].ChaosPolicyRefs = nil
	}
	st.Spec.Chaos.Policies = []model.ChaosPolicy{{
		ID: "hooks", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassLow,
		Budget:   &model.ChaosBudget{MaxRate: 5, MaxConcurrency: 2},
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "h", Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{
			{Type: model.ActionPressure, Value: "drop"},
			{Type: model.ActionCache, Value: CacheValueBypass},
			{Type: model.ActionUpstream, Value: UpstreamValueForce, UpstreamID: "u1"},
		}}},
	}}
	snap := compileSnap(t, st)
	eng := NewEngine(testutil.NewFakeClock(time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)), nil)
	plan, err := eng.Decide(context.Background(), snap, DecisionIn{
		Query: model.Query{Name: "x.example.", Type: model.TypeA, Transport: model.TransportUDP},
		Phase: PhasePreResolution,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Cache.Bypass {
		t.Fatalf("cache %+v", plan.Cache)
	}
	if plan.Upstream.Force != "u1" {
		t.Fatalf("upstream %+v", plan.Upstream)
	}
	if plan.Pressure.PolicyID != "hooks" || plan.Pressure.OnExceed != "drop" || plan.Pressure.MaxConc != 2 {
		t.Fatalf("pressure %+v", plan.Pressure)
	}
}

func hasSkip(plan ActionPlan, id model.PolicyID, reason string) bool {
	for _, d := range plan.Decisions {
		if d.PolicyID == id && d.SkipReason == reason {
			return true
		}
	}
	return false
}

func mustSnap(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	return compileSnap(t, sampleState(t))
}

func compileSnap(t *testing.T, st *model.State) *snapshot.Snapshot {
	t.Helper()
	idx, err := Compile(st)
	if err != nil {
		t.Fatal(err)
	}
	return &snapshot.Snapshot{
		Canonical:  st,
		Revision:   "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CompiledAt: time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC),
		Chaos:      idx,
		Safety: snapshot.SafetyPolicy{
			ProtectedNames:        append([]model.Name(nil), st.Spec.Chaos.Safety.ProtectedNames...),
			ProtectedClientGroups: append([]model.ClientGroupID(nil), st.Spec.Chaos.Safety.ProtectedClientGroups...),
			MaxDelay:              st.Spec.Chaos.Safety.MaxDelay,
			MaxConcurrentDelayed:  st.Spec.Chaos.Safety.MaxConcurrentDelayed,
			MaxDropProbability:    st.Spec.Chaos.Safety.MaxDropProbability,
		},
	}
}
