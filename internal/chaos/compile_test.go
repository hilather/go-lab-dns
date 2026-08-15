package chaos

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestCompileNilAndEmpty(t *testing.T) {
	idx, err := Compile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !idx.Compiled() || idx.Enabled {
		t.Fatalf("nil state: compiled=%v enabled=%v", idx.Compiled(), idx.Enabled)
	}
	idx, err = Compile(&model.State{})
	if err != nil {
		t.Fatal(err)
	}
	if !idx.Compiled() {
		t.Fatal("empty state not compiled")
	}
}

func TestCompileIndexesAndRefs(t *testing.T) {
	st := sampleState(t)
	idx, err := Compile(st)
	if err != nil {
		t.Fatal(err)
	}
	if !idx.Enabled {
		t.Fatal("enabled")
	}
	p, ok := idx.Lookup("slow-tools")
	if !ok || p.Precedence != snapshot.ChaosPrecRecord {
		t.Fatalf("slow-tools %+v", p)
	}
	ids := idx.ByRecord["tools-wildcard-a"]
	if len(ids) != 1 || ids[0] != "slow-tools" {
		t.Fatalf("ByRecord=%v", ids)
	}
	g, ok := idx.Lookup("global-delay")
	if !ok || g.Precedence != snapshot.ChaosPrecGlobal {
		t.Fatalf("global %+v", g)
	}
	if len(idx.Global) != 1 || idx.Global[0] != "global-delay" {
		t.Fatalf("Global=%v", idx.Global)
	}
}

func TestCompileHighImpactCap(t *testing.T) {
	st := sampleState(t)
	st.Spec.Chaos.Safety.MaxActiveHighImpactPolicies = 1
	exp := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	st.Spec.Chaos.Policies = append(st.Spec.Chaos.Policies, model.ChaosPolicy{
		ID: "hi1", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassHigh, ExpiresAt: &exp,
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "x", Weight: 1, Actions: []model.ChaosAction{{Type: model.ActionDelay, Duration: time.Millisecond}}}},
	}, model.ChaosPolicy{
		ID: "hi2", Owner: "o", Reason: "r", Enabled: true, SafetyClass: model.SafetyClassHigh, ExpiresAt: &exp,
		Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Probability: 1},
		Outcomes: []model.ChaosOutcome{{ID: "x", Weight: 1, Actions: []model.ChaosAction{{Type: model.ActionDelay, Duration: time.Millisecond}}}},
	})
	_, err := Compile(st)
	if err == nil {
		t.Fatal("expected cap error")
	}
	if de, ok := domainerr.As(err); !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("err=%v", err)
	}
}

func TestCompileDuplicateID(t *testing.T) {
	st := sampleState(t)
	st.Spec.Chaos.Policies = append(st.Spec.Chaos.Policies, st.Spec.Chaos.Policies[0])
	if _, err := Compile(st); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func sampleState(t *testing.T) *model.State {
	t.Helper()
	return &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabDNS,
		Metadata:   model.Metadata{Name: "t"},
		Spec: model.Spec{
			Access: model.AccessSpec{
				UnknownClient: model.UnknownClientRefuseForward,
				ClientGroups: []model.ClientGroup{
					{ID: "test-devices", CIDRs: []string{"10.42.0.0/16"}, AllowForward: true},
					{ID: "management", CIDRs: []string{"10.42.255.0/24"}, ChaosExempt: true, AllowForward: true},
				},
			},
			Zones: []model.Zone{{
				ID: "lab-zone", Name: "lab.example.net.", Mode: model.ZoneModeAuthoritative,
				Records: []model.Record{
					{ID: "tools-wildcard-a", Owner: "*.tools.lab.example.net.", Type: model.TypeA, Values: []string{"10.42.0.20"}, ChaosPolicyRefs: []model.PolicyID{"slow-tools"}},
					{ID: "ns1-a", Owner: "ns1.lab.example.net.", Type: model.TypeA, Values: []string{"10.42.0.53"}},
				},
			}},
			Chaos: model.ChaosSpec{
				Enabled: true,
				Safety: model.SafetySpec{
					ProtectedNames:        []model.Name{"dns.lab.example.net."},
					ProtectedClientGroups: []model.ClientGroupID{"management"},
					MaxDelay:              10 * time.Second,
					MaxConcurrentDelayed:  8,
					MaxDropProbability:    0.5,
				},
				Policies: []model.ChaosPolicy{
					{
						ID: "slow-tools", Owner: "platform-lab", Reason: "test", Enabled: true,
						SafetyClass: model.SafetyClassLow,
						Scope:       model.ChaosScope{RecordIDs: []model.RecordID{"tools-wildcard-a"}, ClientGroups: []model.ClientGroupID{"test-devices"}},
						Selector:    model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "startup-v1", Probability: 1, TimeBucket: time.Second},
						Outcomes: []model.ChaosOutcome{{
							ID: "delayed", Weight: 1,
							Actions: []model.ChaosAction{{Type: model.ActionDelay, Phase: model.PhaseBeforeResponse, Distribution: model.DistUniform, Min: 100 * time.Millisecond, Max: 750 * time.Millisecond}},
						}},
					},
					{
						ID: "global-delay", Owner: "platform-lab", Reason: "test", Enabled: true,
						SafetyClass: model.SafetyClassLow, Composition: model.CompositionCompose,
						Selector: model.ChaosSelector{Mode: model.SelectorDeterministic, Seed: "g", Probability: 1},
						Outcomes: []model.ChaosOutcome{{
							ID: "d", Weight: 1,
							Actions: []model.ChaosAction{{Type: model.ActionDelay, Phase: model.PhaseBeforeResolution, Distribution: model.DistFixed, Duration: 5 * time.Millisecond}},
						}},
					},
				},
			},
		},
	}
}
