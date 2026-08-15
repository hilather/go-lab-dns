package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestChaosActivateStubs(t *testing.T) {
	path := copyFixture(t)
	svc, _ := mustBoot(t, path)
	ctx := context.Background()
	_, err := svc.ActivateChaos(ctx, actor(), ActivationIn{PolicyID: "slow-tools"})
	requireCode(t, err, domainerr.CodeUnsupportedCapability)
	_, err = svc.DeactivateChaos(ctx, actor(), ActivationIn{PolicyID: "slow-tools"})
	requireCode(t, err, domainerr.CodeUnsupportedCapability)
	_, err = svc.SetChaosExpiry(ctx, actor(), ExpiryIn{PolicyID: "slow-tools"})
	requireCode(t, err, domainerr.CodeUnsupportedCapability)
	_, err = svc.SimulateChaos(ctx, actor(), SimulateIn{})
	requireCode(t, err, domainerr.CodeUnsupportedCapability)
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
	requireCode(t, err, domainerr.CodeNotFound)
}
