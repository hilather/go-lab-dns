package effects

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func TestHintMapping(t *testing.T) {
	cases := []struct {
		name string
		hint string
		tr   model.Transport
		hold time.Duration
		want dnsserver.TransportHint
	}{
		{"udp-drop", "drop", model.TransportUDP, 0, dnsserver.HintDrop},
		{"tcp-drop-hold", "drop", model.TransportTCP, 0, dnsserver.HintHoldThenClose},
		{"udp-tc", "truncate", model.TransportUDP, 0, dnsserver.HintTruncate},
		{"tcp-close", "tcp-close", model.TransportTCP, 0, dnsserver.HintTCPClose},
		{"tcp-close-hold", "tcp-close", model.TransportTCP, time.Second, dnsserver.HintHoldThenClose},
		{"tcp-reset", "tcp-reset", model.TransportTCP, 0, dnsserver.HintTCPReset},
		{"send", "", model.TransportUDP, 0, dnsserver.HintSend},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Hint(chaos.ActionPlan{TransportHint: tc.hint, Hold: tc.hold}, tc.tr, nil)
			if got != tc.want {
				t.Fatalf("got=%s want=%s", got, tc.want)
			}
		})
	}
}

func TestTransportPlanPrecedenceAcrossPhases(t *testing.T) {
	snap := &snapshot.Snapshot{Canonical: &model.State{Spec: model.Spec{Chaos: model.ChaosSpec{Policies: []model.ChaosPolicy{{ID: "early"}, {ID: "late"}}}}}}
	makePlan := func(id model.PolicyID, precedence int, hint string) chaos.ActionPlan {
		return chaos.ActionPlan{TransportHint: hint, Decisions: []chaos.PolicyDecision{{PolicyID: id, Precedence: precedence, Triggered: true, Actions: []chaos.PlannedAction{{Type: hint}}}}}
	}
	for _, tc := range []struct {
		name      string
		pre, post chaos.ActionPlan
		want      string
	}{
		{"pre-specific", makePlan("early", 3, "drop"), makePlan("late", 8, "truncate"), "drop"},
		{"post-specific", makePlan("early", 8, "drop"), makePlan("late", 3, "truncate"), "truncate"},
		{"config-order", makePlan("late", 8, "drop"), makePlan("early", 8, "truncate"), "truncate"},
		{"single", chaos.ActionPlan{}, makePlan("early", 8, "truncate"), "truncate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SelectTransportPlan(snap, tc.pre, tc.post); got.TransportHint != tc.want {
				t.Fatalf("hint=%s want %s", got.TransportHint, tc.want)
			}
		})
	}
}
