package app

import (
	"context"
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestBatchHighImpactActivationRequiresEmergencyScope(t *testing.T) {
	for _, method := range []string{"plan", "apply", "validate"} {
		t.Run(method, func(t *testing.T) {
			svc, boot := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
			p := boot.Canonical.Spec.Chaos.Policies[0]
			p.ID = "batch-high"
			p.SafetyClass = model.SafetyClassHigh
			expiry := boot.CompiledAt.Add(time.Hour)
			ops := []model.Operation{
				{Op: model.OpAdd, Target: model.Target{Kind: model.TargetChaosPolicy, ID: string(p.ID)}, Value: mustJSON(p)},
				{Op: model.OpUpdate, Target: model.Target{Kind: model.TargetChaosActivation, ID: string(p.ID)}, Value: mustJSON(model.ChaosActivation{Enabled: true, ExpiresAt: &expiry})},
			}
			limited := Actor{ID: "designer-operator", Class: auth.ClassToken, Scopes: []string{auth.ScopeChaosWrite, auth.ScopeChaosActivate}}
			in := ChangeIn{ExpectedRevision: boot.Revision, Operations: ops}
			var err error
			switch method {
			case "plan":
				_, err = svc.Plan(context.Background(), limited, in)
			case "apply":
				_, err = svc.Apply(context.Background(), limited, in)
			case "validate":
				_, err = svc.Validate(context.Background(), limited, ValidateIn{Operations: ops})
			}
			_ = requireCode(t, err, domainerr.CodeForbidden)
			if svc.Store().Load() != boot {
				t.Fatal("denied batch changed active snapshot")
			}
			if _, err := svc.Apply(context.Background(), actor(), in); err != nil {
				t.Fatalf("administrator batch: %v", err)
			}
		})
	}
}

func TestBatchProtectedRelativeOwnerInAddedZone(t *testing.T) {
	for _, method := range []string{"plan", "apply", "validate"} {
		t.Run(method, func(t *testing.T) {
			svc, boot := mustBoot(t, copyNamedFixture(t, "pack-sample.yaml"))
			ops := []model.Operation{
				{Op: model.OpAdd, Target: model.Target{Kind: model.TargetZone, ID: "parent-overlay"}, Value: mustJSON(model.Zone{ID: "parent-overlay", Name: "example.net.", Mode: model.ZoneModeOverlay})},
				{Op: model.OpAdd, Target: model.Target{Kind: model.TargetRecord, ID: "protected-a", ZoneID: "parent-overlay"}, Value: mustJSON(model.Record{ID: "protected-a", Owner: "dns.lab", Type: model.TypeA, Values: []string{"10.42.0.99"}})},
			}
			editor := Actor{ID: "editor", Class: auth.ClassToken, Role: auth.RoleDNSEditor}
			in := ChangeIn{ExpectedRevision: boot.Revision, Operations: ops}
			var err error
			switch method {
			case "plan":
				_, err = svc.Plan(context.Background(), editor, in)
			case "apply":
				_, err = svc.Apply(context.Background(), editor, in)
			case "validate":
				_, err = svc.Validate(context.Background(), editor, ValidateIn{Operations: ops})
			}
			_ = requireCode(t, err, domainerr.CodeProtectedObject)
			if svc.Store().Load() != boot {
				t.Fatal("denied batch changed active snapshot")
			}
		})
	}
}
