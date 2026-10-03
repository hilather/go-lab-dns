package app

import (
	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/model"
	"net/netip"
	"testing"
)

func TestManagementResolveAndExplainModelActiveChaosWithoutSideEffects(t *testing.T) {
	svc, boot := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	svc.cache = cache.New(cache.PolicyFromSpec(boot.Canonical.Spec.Cache), svc.clock)
	policy := boot.Canonical.Spec.Chaos.Policies[0]
	policy.Enabled = true
	policy.Outcomes[0].Actions = append(policy.Outcomes[0].Actions, model.ChaosAction{Type: model.ActionTTL, Phase: model.PhaseBeforeResponse, Value: "zero"})
	_, err := svc.Apply(t.Context(), actor(), ChangeIn{ExpectedRevision: boot.Revision, Operations: []model.Operation{{Op: model.OpUpdate, Target: model.Target{Kind: model.TargetChaosPolicy, ID: string(policy.ID)}, Value: mustJSON(policy)}}})
	if err != nil {
		t.Fatal(err)
	}
	input := ResolveIn{Name: "x.tools.lab.example.net.", Type: model.TypeA, ClientGroup: "test-devices", ApplyChaos: true, UseCache: true}
	resolved, err := svc.Resolve(t.Context(), actor(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Result.Answers) != 1 || resolved.Result.Answers[0].TTL != 0 {
		t.Fatalf("applyChaos ignored: %+v", resolved.Result)
	}
	before := svc.cache.Stats()
	explained, err := svc.Explain(t.Context(), actor(), input)
	if err != nil {
		t.Fatal(err)
	}
	if explained.Explanation == nil || len(explained.Explanation.ChaosPolicyIDs) == 0 || explained.Result.Answers[0].TTL != 0 {
		t.Fatalf("Explain omitted chaos: %+v", explained)
	}
	if svc.cache.Stats() != before {
		t.Fatal("Explain mutated live cache")
	}
	input.ApplyChaos = false
	base, err := svc.Resolve(t.Context(), actor(), input)
	if err != nil {
		t.Fatal(err)
	}
	if base.Result.Answers[0].TTL == 0 {
		t.Fatal("simulated answer poisoned base cache")
	}
	if svc.engine.Stats().TTL.Load() != 0 || svc.engine.Stats().Delayed.Load() != 0 {
		t.Fatal("management simulation changed live effect counters")
	}
}

func TestCachedManagementResolutionUsesCurrentClientContext(t *testing.T) {
	svc, boot := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	svc.cache = cache.New(cache.PolicyFromSpec(boot.Canonical.Spec.Cache), svc.clock)
	policy := boot.Canonical.Spec.Chaos.Policies[0]
	policy.Enabled = true
	policy.Scope.Transports = []model.Transport{model.TransportUDP}
	policy.Outcomes[0].Actions = []model.ChaosAction{{Type: model.ActionTTL, Phase: model.PhaseBeforeResponse, Value: "zero"}}
	_, err := svc.Apply(t.Context(), actor(), ChangeIn{ExpectedRevision: boot.Revision, Operations: []model.Operation{{Op: model.OpUpdate, Target: model.Target{Kind: model.TargetChaosPolicy, ID: string(policy.ID)}, Value: mustJSON(policy)}}})
	if err != nil {
		t.Fatal(err)
	}
	first := ResolveIn{Name: "x.tools.lab.example.net.", Type: model.TypeA, Client: netip.MustParseAddr("10.42.0.10"), Transport: model.TransportUDP, UseCache: true, ApplyChaos: true}
	if _, err := svc.Resolve(t.Context(), actor(), first); err != nil {
		t.Fatal(err)
	}
	for _, current := range []ResolveIn{
		{Name: first.Name, Type: first.Type, Client: netip.MustParseAddr("10.42.255.9"), Transport: model.TransportUDP, UseCache: true, ApplyChaos: true},
		{Name: first.Name, Type: first.Type, Client: netip.MustParseAddr("10.42.255.9"), Transport: model.TransportTCP, UseCache: true, ApplyChaos: true},
		{Name: first.Name, Type: first.Type, Transport: model.TransportTCP, UseCache: true, ApplyChaos: true},
	} {
		result, err := svc.Resolve(t.Context(), actor(), current)
		if err != nil {
			t.Fatal(err)
		}
		explanation := result.Result.Explanation
		if explanation.Query.Client != current.Client || explanation.Query.Transport != current.Transport {
			t.Fatalf("cached previous caller leaked: %+v", explanation)
		}
		expectedGroup := model.ClientGroupID("")
		if current.Client.IsValid() {
			expectedGroup = "management"
		}
		if explanation.ClientGroupID != expectedGroup {
			t.Fatalf("cached group=%q want %q", explanation.ClientGroupID, expectedGroup)
		}
		if result.Result.Answers[0].TTL == 0 || len(explanation.ChaosPolicyIDs) > 0 {
			t.Fatalf("previous caller chaos context reused: %+v", result.Result)
		}
	}
}

func TestManagementChaosModelRespectsDisabledPolicies(t *testing.T) {
	svc, _ := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	result, err := svc.Explain(t.Context(), actor(), ResolveIn{Name: "x.tools.lab.example.net.", Type: model.TypeA, ClientGroup: "test-devices"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Explanation.ChaosPolicyIDs) != 0 {
		t.Fatalf("disabled policies modeled as active: %+v", result.Explanation)
	}
	for _, decision := range result.Explanation.ChaosDecisions {
		if decision.Triggered || decision.SkipReason != "disabled" || decision.Delay != 0 {
			t.Fatalf("disabled policy decision=%+v", decision)
		}
	}
}
