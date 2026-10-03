package dnsquery

import (
	"context"
	"net/netip"
	"testing"

	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestTransportPrecedenceAcrossResolutionPhases(t *testing.T) {
	for _, tr := range []model.Transport{model.TransportUDP, model.TransportTCP} {
		t.Run(string(tr), func(t *testing.T) {
			st := chaosState(t)
			st.Spec.Chaos.Policies = nil
			for _, spec := range []struct {
				id            model.PolicyID
				phase, action string
				specific      bool
			}{{"specific-pre", model.PhaseBeforeResolution, model.ActionDrop, true}, {"global-post", model.PhaseBeforeResponse, model.ActionTruncate, false}} {
				p := model.ChaosPolicy{ID: spec.id, Owner: "lab", Reason: "phase precedence", Enabled: true, SafetyClass: model.SafetyClassLow, Selector: model.ChaosSelector{Probability: 1}, Outcomes: []model.ChaosOutcome{{ID: "o", Weight: 1, Actions: []model.ChaosAction{{Type: spec.action, Phase: spec.phase}}}}}
				if spec.specific {
					p.Scope.Owners = []model.Name{"ns.lab.example."}
				}
				st.Spec.Chaos.Policies = append(st.Spec.Chaos.Policies, p)
			}
			h := handlerFromState(t, st, nil)
			q := model.Query{Name: "ns.lab.example.", Type: model.TypeA, Class: model.ClassIN, Client: netip.MustParseAddr("10.42.0.10"), Transport: tr}
			_, hint, err := h.ServeDNS(context.Background(), &q)
			if err != nil {
				t.Fatal(err)
			}
			want := dnsserver.HintDrop
			if tr == model.TransportTCP {
				want = dnsserver.HintHoldThenClose
			}
			if hint != want {
				t.Fatalf("hint=%s want %s", hint, want)
			}
		})
	}
}
