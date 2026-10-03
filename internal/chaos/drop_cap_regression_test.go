package chaos

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

func dropCapState(t *testing.T, cap float64, policies ...model.ChaosPolicy) *model.State {
	t.Helper()
	state := sampleState(t)
	for z := range state.Spec.Zones {
		for r := range state.Spec.Zones[z].Records {
			state.Spec.Zones[z].Records[r].ChaosPolicyRefs = nil
		}
	}
	state.Spec.Chaos.Policies = policies
	state.Spec.Chaos.Safety.MaxDropProbability = cap
	return state
}
func dropPolicy(id model.PolicyID, phases ...string) model.ChaosPolicy {
	actions := []model.ChaosAction{}
	for _, phase := range phases {
		actions = append(actions, model.ChaosAction{Type: model.ActionDrop, Phase: phase})
	}
	return model.ChaosPolicy{ID: id, Enabled: true, Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: string(id), Probability: 1}, Outcomes: []model.ChaosOutcome{{ID: "drop", Weight: 1, Actions: actions}}}
}
func TestAggregateDropAllocationAcrossPoliciesAndPhases(t *testing.T) {
	for _, both := range []bool{false, true} {
		t.Run(fmt.Sprint(both), func(t *testing.T) {
			first := dropPolicy("first", model.PhaseBeforeResolution)
			if both {
				first = dropPolicy("first", model.PhaseBeforeResolution, model.PhaseBeforeResponse)
			}
			second := dropPolicy("second", model.PhaseBeforeResponse)
			state := dropCapState(t, .5, first, second)
			before, _ := json.Marshal(state)
			snap := compileSnap(t, state)
			charge := snap.Chaos.ByID["first"].Policy.Selector.Probability + snap.Chaos.ByID["second"].Policy.Selector.Probability
			if both {
				charge += snap.Chaos.ByID["first"].Policy.Selector.Probability
			}
			if charge > .5 {
				t.Fatalf("composed phase selector bound=%g exceeds .5", charge)
			}
			after, _ := json.Marshal(state)
			if string(before) != string(after) {
				t.Fatal("compilation changed canonical export")
			}
		})
	}
}
func TestDropAllocationCompatibilityAndWeightedPressure(t *testing.T) {
	for _, cap := range []float64{0, 1, .5} {
		policy := dropPolicy("single", model.PhaseBeforeResponse)
		snap := compileSnap(t, dropCapState(t, cap, policy))
		expected := 1.0
		if cap == .5 {
			expected = .5
		}
		if got := snap.Chaos.ByID["single"].Policy.Selector.Probability; got != expected {
			t.Fatalf("cap%g single=%g", cap, got)
		}
	}
	pressure := dropPolicy("pressure", model.PhaseBeforeResolution)
	pressure.Enabled = false
	pressure.Outcomes[0].Actions = []model.ChaosAction{{Type: model.ActionPressure, Value: "drop"}}
	weighted := dropPolicy("weighted", model.PhaseBeforeResponse)
	weighted.Outcomes = append(weighted.Outcomes, model.ChaosOutcome{ID: "normal", Weight: 99})
	snap := compileSnap(t, dropCapState(t, .4, pressure, weighted))
	for _, id := range []model.PolicyID{"pressure", "weighted"} {
		if got := snap.Chaos.ByID[id].Policy.Selector.Probability; math.Abs(got-.2) > 1e-15 {
			t.Fatalf("%s probability=%g", id, got)
		}
	}
	for _, cap := range []float64{0, 1} {
		snap := compileSnap(t, dropCapState(t, cap, pressure, weighted))
		for _, cp := range snap.Chaos.ByID {
			if cp.Policy.Selector.Probability != 1 {
				t.Fatalf("cap%g changed selector", cap)
			}
		}
	}
}
func TestAggregateDropDeterministicSimulationParity(t *testing.T) {
	snap := compileSnap(t, dropCapState(t, .4, dropPolicy("pre", model.PhaseBeforeResolution), dropPolicy("post", model.PhaseBeforeResponse)))
	engine := NewEngine(testutil.NewFakeClock(snap.CompiledAt), nil)
	drops := 0
	for i := 0; i < 1000; i++ {
		anyDrop := false
		for _, phase := range []Phase{PhasePreResolution, PhaseResponse} {
			in := DecisionIn{Query: model.Query{Name: model.Name(fmt.Sprintf("q%d.example.", i)), Type: model.TypeA, Transport: model.TransportUDP}, Phase: phase}
			live, err := engine.Decide(t.Context(), snap, in)
			if err != nil {
				t.Fatal(err)
			}
			sim, err := engine.Simulate(t.Context(), snap, SimulateIn{Query: in.Query, Phase: phase})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(live.Decisions, sim.Decisions) {
				t.Fatal("live/simulation decision differs")
			}
			if live.TransportHint == "drop" {
				anyDrop = true
			}
		}
		if anyDrop {
			drops++
		}
	}
	if drops > 450 {
		t.Fatalf("aggregate observed drops=%d/1000 exceeds cap tolerance", drops)
	}
}

type countingDropRand struct{ draws int }

func (r *countingDropRand) Uint64() uint64 { r.draws++; return uint64(1) << 63 }
func TestAggregateDropRandomStickyUsesOriginalDrawCount(t *testing.T) {
	policy := dropPolicy("both", model.PhaseBeforeResolution, model.PhaseBeforeResponse)
	policy.Selector.Mode = model.SelectorRandom
	snap := compileSnap(t, dropCapState(t, .5, policy))
	rng := &countingDropRand{}
	engine := NewEngine(testutil.NewFakeClock(snap.CompiledAt), rng)
	sticky := NewStickyRand()
	for _, phase := range []Phase{PhasePreResolution, PhaseResponse} {
		plan, err := engine.Decide(t.Context(), snap, DecisionIn{Query: model.Query{Name: "q.example.", Type: model.TypeA, Transport: model.TransportUDP}, Phase: phase, Sticky: sticky})
		if err != nil {
			t.Fatal(err)
		}
		if !hasSkip(plan, "both", "probability") {
			t.Fatalf("expected capped probability skip: %+v", plan)
		}
	}
	if rng.draws != 3 {
		t.Fatalf("draw count=%d want3", rng.draws)
	}
}

func TestDropAllocationAccountsForDeterministicBucketBoundary(t *testing.T) {
	policy := dropPolicy("boundary", model.PhaseBeforeResolution, model.PhaseBeforeResponse)
	policy.Selector.TimeBucket = 1000000000
	snap := compileSnap(t, dropCapState(t, .5, policy))
	probability := snap.Chaos.ByID[policy.ID].Policy.Selector.Probability
	if probability*2 > .5 {
		t.Fatalf("two independently bucketed phase events exceed cap: %g", probability*2)
	}
	// Find a deterministic witness where selector-only allocation (.5) would
	// drop before resolution, while the correctly allocated threshold does not.
	clk := testutil.NewFakeClock(snap.CompiledAt)
	engine := NewEngine(clk, nil)
	for i := 0; i < 1000; i++ {
		query := model.Query{Name: model.Name(fmt.Sprintf("boundary%d.example.", i)), Type: model.TypeA, Transport: model.TransportUDP}
		before, err := engine.Decide(t.Context(), snap, DecisionIn{Query: query, Phase: PhasePreResolution})
		if err != nil {
			t.Fatal(err)
		}
		clk.Advance(1000000000)
		after, err := engine.Decide(t.Context(), snap, DecisionIn{Query: query, Phase: PhaseResponse})
		if err != nil {
			t.Fatal(err)
		}
		a, b := before.Decisions[0], after.Decisions[0]
		if a.Hash.P > probability && a.Hash.P < .5 && b.Hash.P >= .5 {
			if a.Triggered || b.Triggered || a.Hash.DigestHex == b.Hash.DigestHex {
				t.Fatalf("boundary decisions=%+v/%+v", a, b)
			}
			if len(before.Clamped) != 1 || before.Clamped[0].Reason != "max_drop_probability" {
				t.Fatal("missing effective threshold evidence")
			}
			return
		}
	}
	t.Fatal("deterministic bucket-boundary witness not found")
}
