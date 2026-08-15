package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-dns/internal/forwarder"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/observability"
)

func TestStatusFillsViewsAndWarnings(t *testing.T) {
	path := copyNamedFixture(t, "pack-sample.yaml")
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	st, err := svc.Status(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Ready || st.Degraded {
		t.Fatalf("ready=%v degraded=%v", st.Ready, st.Degraded)
	}
	if st.Revisions.RuntimeRevision != boot.Revision || st.Revisions.BootstrapRevision != boot.Revision {
		t.Fatalf("revisions=%+v boot=%s", st.Revisions, boot.Revision)
	}
	if len(st.Listeners) < 2 {
		t.Fatalf("listeners=%v", st.Listeners)
	}
	if !st.Chaos.Enabled {
		t.Fatal("pack-sample chaos is enabled with inactive policies")
	}
	if st.Chaos.EmergencyDisabled {
		t.Fatal("pack-sample must start without emergency inhibit")
	}
	if len(st.Upstreams) == 0 {
		t.Fatal("expected upstreams")
	}
}

func TestStatusDegradedOnUnhealthyUpstream(t *testing.T) {
	path := copyNamedFixture(t, "pack-sample.yaml")
	svc, _ := mustBoot(t, path)
	h := forwarder.NewHealth(nil)
	h.RecordFailure("corp-1")
	h.RecordFailure("corp-1")
	svc.health = h
	st, err := svc.Status(context.Background(), actor())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Ready || !st.Degraded {
		t.Fatalf("ready=%v degraded=%v", st.Ready, st.Degraded)
	}
	if !hasWarning(st.Warnings, observability.WarnUpstreamUnhealthy) {
		t.Fatalf("warnings=%v", st.Warnings)
	}
}

func TestStatusChaosDoesNotUnready(t *testing.T) {
	path := copyNamedFixture(t, "pack-sample.yaml")
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.EmergencyDisableChaos(ctx, actor(), EmergencyIn{Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Status(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Ready || st.Degraded {
		t.Fatalf("chaos emergency must not flip ready/degraded: %+v", st)
	}
	if !st.Chaos.EmergencyDisabled {
		t.Fatal("expected emergency bit")
	}
	if !hasWarning(st.Warnings, observability.WarnChaosEmergency) {
		t.Fatalf("warnings=%v", st.Warnings)
	}
	if st.Revisions.RuntimeRevision != boot.Revision {
		t.Fatalf("emergency changed revision %s vs %s", st.Revisions.RuntimeRevision, boot.Revision)
	}
}

func TestStatusDriftWarning(t *testing.T) {
	path := copyFixture(t)
	svc, boot := mustBoot(t, path)
	ctx := context.Background()
	if _, err := svc.Apply(ctx, actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Reason:           "drift",
		Operations:       []model.Operation{addWWWRecord()},
	}); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Status(ctx, actor())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Revisions.Drifted || !hasWarning(st.Warnings, observability.WarnStateDrifted) {
		t.Fatalf("status=%+v", st)
	}
}

type stubHealth struct {
	process, dns, mgmt bool
	drops              int64
}

func (s stubHealth) ProcessDown() bool     { return s.process }
func (s stubHealth) DNSDown() bool         { return s.dns }
func (s stubHealth) MgmtDown() bool        { return s.mgmt }
func (s stubHealth) TelemetryDrops() int64 { return s.drops }

func TestStatusUnreadyWhenListenerDown(t *testing.T) {
	path := copyFixture(t)
	svc, _ := mustBoot(t, path)
	svc.healthSrc = stubHealth{dns: true}
	st, err := svc.Status(context.Background(), actor())
	if err != nil {
		t.Fatal(err)
	}
	if st.Ready {
		t.Fatal("dns down must be unready")
	}
	if !hasWarning(st.Warnings, observability.WarnListenerUnbound) {
		t.Fatalf("warnings=%v", st.Warnings)
	}
}

func hasWarning(ws []Warning, code string) bool {
	for _, w := range ws {
		if w.Code == code {
			return true
		}
	}
	return false
}
