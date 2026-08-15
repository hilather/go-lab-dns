package effects

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestDropWinsOverRCodeOnWire(t *testing.T) {
	plan := chaos.ActionPlan{
		TransportHint: "drop",
		EarlyRCode:    model.RCodeServFail,
		Actions: []chaos.PlannedAction{
			{Type: model.ActionRCode, RCode: "SERVFAIL"},
			{Type: model.ActionDrop, Value: "drop"},
		},
	}
	if Hint(plan, model.TransportUDP, nil) != dnsserver.HintDrop {
		t.Fatal("drop must win the transport")
	}
	res := ApplyResponse(model.Result{RCode: model.RCodeNoError, Answers: []model.RR{{Data: "1"}}}, plan, model.Query{}, nil)
	if res.RCode != model.RCodeServFail {
		t.Fatalf("explanation path still applies rcode, got %s", res.RCode)
	}
}

func TestCacheBypassBeatsStale(t *testing.T) {
	opts := CacheGet(chaos.ActionPlan{Cache: chaos.CachePlan{Bypass: true, ServeStale: true, Expire: true}}, nil)
	if !opts.Bypass {
		t.Fatal("bypass")
	}
}

func TestAnnotateBaseAndFinal(t *testing.T) {
	base := model.Result{RCode: model.RCodeNoError, Explanation: &model.Explanation{}}
	res := model.Result{RCode: model.RCodeServFail, Explanation: &model.Explanation{}}
	Annotate(&res, base, chaos.ActionPlan{
		Actions:   []chaos.PlannedAction{{Type: model.ActionRCode, PolicyID: "p"}},
		Decisions: []chaos.PolicyDecision{{PolicyID: "p", Triggered: true}},
	})
	if res.Explanation.BaseRCode != model.RCodeNoError {
		t.Fatalf("base=%s", res.Explanation.BaseRCode)
	}
	if len(res.Explanation.ChaosPolicyIDs) != 1 || res.Explanation.ChaosActions[0] != model.ActionRCode {
		t.Fatalf("%+v", res.Explanation)
	}
}

func TestTTLJitterStaysInRange(t *testing.T) {
	base := model.Result{Answers: []model.RR{{TTL: 10 * time.Second}}}
	got := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionTTL, Value: chaos.TTLValueJitter, Min: 0, Max: 2 * time.Second, Seed: 1,
	}}}, model.Query{}, nil)
	if got.Answers[0].TTL < 10*time.Second || got.Answers[0].TTL > 12*time.Second {
		t.Fatalf("jitter %s", got.Answers[0].TTL)
	}
}
