package app

import (
	"github.com/hilather/go-lab-dns/internal/cache"
	"github.com/hilather/go-lab-dns/internal/model"
	"testing"
)

func TestCachePolicyApplyAndReset(t *testing.T) {
	svc, boot := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
	svc.cache = cache.New(cache.PolicyFromSpec(boot.Canonical.Spec.Cache), svc.clock)
	input := ResolveIn{Name: "ns1.lab.example.net.", Type: model.TypeA, UseCache: true}
	if _, err := svc.Resolve(t.Context(), actor(), input); err != nil {
		t.Fatal(err)
	}
	disabled := boot.Canonical.Spec.Cache
	disabled.Enabled = false
	_, err := svc.Apply(t.Context(), actor(), ChangeIn{ExpectedRevision: boot.Revision, Operations: []model.Operation{{Op: model.OpUpdate, Target: model.Target{Kind: model.TargetCache}, Value: mustJSON(disabled)}}})
	if err != nil {
		t.Fatal(err)
	}
	status, err := svc.CacheStatus(t.Context(), actor())
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.Entries != 0 {
		t.Fatalf("disabled runtime policy ignored: %+v", status)
	}
	if _, err := svc.Reset(t.Context(), actor(), ResetIn{}); err != nil {
		t.Fatal(err)
	}
	status, err = svc.CacheStatus(t.Context(), actor())
	if err != nil || !status.Enabled || status.MaxEntries != boot.CachePolicy.MaxEntries {
		t.Fatalf("reset policy ignored: %+v %v", status, err)
	}
}
