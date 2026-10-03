package app

import (
	"os"
	"testing"

	"github.com/hilather/go-lab-dns/internal/audit"
	"github.com/hilather/go-lab-dns/internal/auth"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestResetCancelsDelaysOnlyAfterSuccessfulCompile(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			path := copyFixture(t)
			svc, boot := mustBoot(t, path)
			canceled := false
			unregister := svc.engine.Budgets().WatchCancel(func() { canceled = true })
			defer unregister()
			if fail {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			_, err := svc.Reset(t.Context(), actor(), ResetIn{})
			if fail {
				if err == nil || canceled || svc.Store().Load() != boot {
					t.Fatalf("failed reset affected live state: %v canceled=%v", err, canceled)
				}
			} else if err != nil || !canceled {
				t.Fatalf("successful reset did not cancel pending delays: %v canceled=%v", err, canceled)
			}
		})
	}
}

func TestCacheFlushAuthorizationAndAudit(t *testing.T) {
	svc, _ := mustBoot(t, copyFixture(t))
	viewer := Actor{ID: "viewer", Class: auth.ClassToken, Role: auth.RoleViewer}
	requireCode(t, svc.CacheFlush(t.Context(), viewer, FlushIn{}), domainerr.CodeForbidden)
	events := svc.Audit().List(1)
	if len(events) != 1 || events[0].Result != audit.ResultDenied || events[0].Capability != "dns_cache_flush" {
		t.Fatalf("missing denied audit: %+v", events)
	}
	if err := svc.CacheFlush(t.Context(), actor(), FlushIn{}); err != nil {
		t.Fatal(err)
	}
	events = svc.Audit().List(1)
	if len(events) != 1 || events[0].Result != audit.ResultOK || events[0].Capability != "dns_cache_flush" || events[0].ActorID != actor().ID {
		t.Fatalf("missing success audit: %+v", events)
	}
}

func TestPolicyLabelsNamedLikeDurationsRemainStrings(t *testing.T) {
	original := model.ChaosPolicy{ID: "p", Labels: map[string]string{"ttl": "lab experiment", "duration": "owner", "min": "50ms"}}
	var decoded model.ChaosPolicy
	if err := decodeValue(mustJSON(original), &decoded, "value"); err != nil {
		t.Fatal(err)
	}
	for key, want := range original.Labels {
		if got := decoded.Labels[key]; got != want {
			t.Fatalf("label %s=%q want %q", key, got, want)
		}
	}
}
