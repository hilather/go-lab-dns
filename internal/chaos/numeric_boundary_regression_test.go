package chaos

import (
	"math"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

type upperEdgeRand struct{}

func (upperEdgeRand) Uint64() uint64 { return math.MaxUint64 }
func TestProbabilityOneTriggersAtRoundedUpperEdge(t *testing.T) {
	for _, probability := range []float64{0, 1} {
		policy := dropPolicy("edge", model.PhaseBeforeResponse)
		policy.Selector.Mode = model.SelectorRandom
		policy.Selector.Probability = probability
		snap := compileSnap(t, dropCapState(t, 1, policy))
		engine := NewEngine(testutil.NewFakeClock(snap.CompiledAt), upperEdgeRand{})
		plan, err := engine.Decide(t.Context(), snap, DecisionIn{Query: model.Query{Name: "q.example.", Type: model.TypeA, Transport: model.TransportUDP}, Phase: PhaseResponse})
		if err != nil {
			t.Fatal(err)
		}
		if got := plan.TransportHint == "drop"; got != (probability == 1) {
			t.Fatalf("probability%g upper-edge triggered=%v", probability, got)
		}
	}
}
func TestUniformDelayKeepsRoundedUpperEdgeInsideRange(t *testing.T) {
	for _, span := range []struct{ min, max time.Duration }{{0, 10}, {5, 10}, {0, time.Duration(math.MaxInt64)}, {10, time.Duration(math.MaxInt64)}} {
		got := UniformDelay(span.min, span.max, math.MaxUint64)
		if got != span.max-1 {
			t.Fatalf("range[%d,%d) got%d want%d", span.min, span.max, got, span.max-1)
		}
	}
	if got := UniformDelay(5, 15, 1<<63); got != 10 {
		t.Fatalf("interior mapping changed:%d", got)
	}
}
func TestPickOutcomeHandlesFiniteWeightSumOverflow(t *testing.T) {
	outcomes := []model.ChaosOutcome{{ID: "zero", Weight: 0}, {ID: "first", Weight: 1e308}, {ID: "negative", Weight: -1}, {ID: "second", Weight: 1e308}}
	for _, tc := range []struct {
		draw float64
		id   string
	}{{.25, "first"}, {.75, "second"}} {
		got, ok := PickOutcome(outcomes, tc.draw)
		if !ok || string(got.ID) != tc.id {
			t.Fatalf("draw%g outcome%s want%s", tc.draw, got.ID, tc.id)
		}
	}
}
