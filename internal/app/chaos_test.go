package app

import (
	"context"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestChaosActivateDeactivateSimulate(t *testing.T) {
	path := copyNamedFixture(t, "pack-sample.yaml")
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.ActivateChaos(ctx, actor(), ActivationIn{PolicyID: "nope", ExpectedRevision: boot.Revision}); err == nil {
		t.Fatal("missing policy")
	} else {
		_ = requireCode(t, err, domainerr.CodeNotFound)
	}
	res, err := svc.ActivateChaos(ctx, actor(), ActivationIn{PolicyID: "slow-tools", ExpectedRevision: boot.Revision, Reason: "lab"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied {
		t.Fatal("not applied")
	}
	live := svc.Store().Load()
	found := false
	for _, p := range live.Canonical.Spec.Chaos.Policies {
		if p.ID == "slow-tools" && p.Enabled {
			found = true
		}
	}
	if !found {
		t.Fatal("slow-tools not enabled")
	}
	sim, err := svc.SimulateChaos(ctx, actor(), SimulateIn{
		Name: "foo.tools.lab.example.net.", Type: model.TypeA,
		ClientGroup: "test-devices", Nonce: "sim",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sim.Algorithm != "hash-v1" {
		t.Fatalf("algo=%s", sim.Algorithm)
	}
	if !sim.Triggered {
		t.Fatalf("expected trigger after activate: %+v", sim)
	}
	exp := live.CompiledAt.Add(time.Hour)
	if _, err := svc.SetChaosExpiry(ctx, actor(), ExpiryIn{PolicyID: "slow-tools", ExpectedRevision: live.Revision, ExpiresAt: &exp}); err != nil {
		t.Fatal(err)
	}
	live = svc.Store().Load()
	if _, err := svc.DeactivateChaos(ctx, actor(), ActivationIn{PolicyID: "slow-tools", ExpectedRevision: live.Revision}); err != nil {
		t.Fatal(err)
	}
	for _, p := range svc.Store().Load().Canonical.Spec.Chaos.Policies {
		if p.ID == "slow-tools" && p.Enabled {
			t.Fatal("still enabled")
		}
	}
}

func TestEmergencyDisableCancelsInFlightDelays(t *testing.T) {
	path := copyFixture(t)
	svc, _ := mustBoot(t, path)
	canceled := make(chan struct{})
	unreg := svc.engine.Budgets().WatchCancel(func() { close(canceled) })
	defer unreg()
	if _, err := svc.EmergencyDisableChaos(context.Background(), actor(), EmergencyIn{Reason: "stop"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("EmergencyDisableChaos did not cancel outstanding delays")
	}
}

func TestEmergencyDisableDoesNotChangeRevision(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if boot.EmergencyChaosOff {
		t.Fatal("fixture compiled with emergency off")
	}
	res, err := svc.EmergencyDisableChaos(ctx, actor(), EmergencyIn{Reason: "stop"})
	if err != nil {
		t.Fatal(err)
	}
	live := svc.Store().Load()
	if live == boot {
		t.Fatal("emergency disable must swap a new snapshot")
	}
	if !live.EmergencyChaosOff {
		t.Fatal("EmergencyChaosOff not set")
	}
	if live.Revision != boot.Revision {
		t.Fatalf("revision changed %s -> %s", boot.Revision, live.Revision)
	}
	if live.Generation != boot.Generation+1 {
		t.Fatalf("generation=%d", live.Generation)
	}
	if !res.Applied {
		t.Fatal("not applied")
	}
	st, err := svc.ChaosStatus(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if !st.EmergencyDisabled {
		t.Fatal("status should report emergency disabled")
	}

	if _, err := svc.EmergencyEnableChaos(ctx, actor(), EmergencyIn{Reason: "resume"}); err != nil {
		t.Fatal(err)
	}
	if svc.Store().Load().EmergencyChaosOff {
		t.Fatal("emergency enable left the bit set")
	}
}

func TestApplyPreservesEmergencyBit(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.EmergencyDisableChaos(ctx, actor(), EmergencyIn{Reason: "stop"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	if !svc.Store().Load().EmergencyChaosOff {
		t.Fatal("apply cleared EmergencyChaosOff")
	}
}

func TestEmergencyDisableDoesNotClobberConcurrentApply(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	svc.afterCompile = func() {
		if _, err := svc.EmergencyDisableChaos(ctx, actor(), EmergencyIn{Reason: "interleave"}); err != nil {
			t.Errorf("emergency during apply: %v", err)
		}
	}
	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	live := svc.Store().Load()
	if !live.EmergencyChaosOff {
		t.Fatal("emergency inhibit missing after interleaved apply")
	}
	found := false
	for _, r := range live.Canonical.Spec.Zones[0].Records {
		if r.ID == "www-a" {
			found = true
		}
	}
	if !found {
		t.Fatal("emergency swap discarded the concurrent apply")
	}
	if live.Revision == boot.Revision {
		t.Fatal("apply revision missing")
	}
}

func TestListChaosPoliciesEmpty(t *testing.T) {
	path := copyFixture(t)
	svc, _ := mustBoot(t, path)
	pols, err := svc.ListChaosPolicies(context.Background(), actor())
	if err != nil {
		t.Fatal(err)
	}
	if len(pols) != 0 {
		t.Fatalf("pols=%+v", pols)
	}
	_, err = svc.GetChaosPolicy(context.Background(), actor(), "nope")
	_ = requireCode(t, err, domainerr.CodeNotFound)
}
