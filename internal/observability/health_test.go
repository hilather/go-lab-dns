package observability

import "testing"

func TestEvaluateHealthSemantics(t *testing.T) {
	ready := Evaluate(Facts{HasRuntimeRevision: true})
	if !ready.Live || !ready.Ready || ready.Degraded {
		t.Fatalf("healthy probe=%+v", ready)
	}

	down := Evaluate(Facts{ProcessDown: true, HasRuntimeRevision: true})
	if down.Live || down.Ready {
		t.Fatalf("process down still live/ready: %+v", down)
	}

	unready := Evaluate(Facts{HasRuntimeRevision: true, DNSDown: true})
	if unready.Ready {
		t.Fatal("dns down must be unready")
	}
	if !hasCode(unready.Warnings, WarnListenerUnbound) {
		t.Fatalf("warnings=%v", unready.Warnings)
	}

	none := Evaluate(Facts{})
	if none.Ready || !hasCode(none.Warnings, WarnNoSnapshot) {
		t.Fatalf("empty facts=%+v", none)
	}

	deg := Evaluate(Facts{HasRuntimeRevision: true, UnhealthyUpstreams: 2})
	if !deg.Ready || !deg.Degraded {
		t.Fatalf("upstream fail should degrade, not unready: %+v", deg)
	}
	if !hasCode(deg.Warnings, WarnUpstreamUnhealthy) {
		t.Fatalf("warnings=%v", deg.Warnings)
	}
}

func TestChaosDoesNotAffectHealth(t *testing.T) {
	base := Evaluate(Facts{HasRuntimeRevision: true})
	with := Evaluate(Facts{
		HasRuntimeRevision: true,
		EmergencyChaos:     true,
		ChaosEnabled:       true,
	})
	if with.Live != base.Live || with.Ready != base.Ready || with.Degraded != base.Degraded {
		t.Fatalf("chaos flipped health base=%+v with=%+v", base, with)
	}
	if !hasCode(with.Warnings, WarnChaosEmergency) {
		t.Fatal("expected informational emergency warning")
	}
}

func TestWarningBound(t *testing.T) {
	p := Evaluate(Facts{
		Drifted:            true,
		UnhealthyUpstreams: 1,
		TelemetryDrops:     3,
		EmergencyChaos:     true,
		CacheEntries:       9,
		CacheMax:           10,
		DNSDown:            true,
	})
	if len(p.Warnings) > MaxWarnings {
		t.Fatalf("warnings=%d", len(p.Warnings))
	}
}

func hasCode(ws []Warning, code string) bool {
	for _, w := range ws {
		if w.Code == code {
			return true
		}
	}
	return false
}
