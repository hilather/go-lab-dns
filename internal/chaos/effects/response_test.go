package effects

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestRCodeNODATAAndErrors(t *testing.T) {
	base := model.Result{
		RCode:   model.RCodeNoError,
		Answers: []model.RR{{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.1"}},
		Authority: []model.RR{{
			Name: "example.", Type: model.TypeSOA, Class: model.ClassIN, TTL: time.Second,
			Data: "ns.example. hostmaster. 1 1 1 1 1",
		}},
	}
	q := model.Query{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN}
	got := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionRCode, RCode: chaos.RCodeNODATA,
	}}}, q, nil)
	if got.RCode != model.RCodeNoError || len(got.Answers) != 0 {
		t.Fatalf("NODATA %+v", got)
	}
	if len(got.Authority) != 1 {
		t.Fatal("NODATA must keep authority")
	}
	if len(base.Answers) != 1 {
		t.Fatal("base answers mutated")
	}

	for _, rc := range []string{"SERVFAIL", "REFUSED", "NXDOMAIN", "FORMERR", "NOTIMP"} {
		got = ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
			Type: model.ActionRCode, RCode: rc, EDE: &model.EDE{Code: 0, Text: "lab"},
		}}}, q, nil)
		if string(got.RCode) != rc {
			t.Fatalf("%s rcode=%s", rc, got.RCode)
		}
		if len(got.Answers) != 0 {
			t.Fatalf("%s kept answers", rc)
		}
		if got.EDE == nil || got.EDE.Text != "lab" {
			t.Fatalf("%s missing EDE", rc)
		}
	}
}

func TestTTLBoundariesAndNoOverflow(t *testing.T) {
	base := model.Result{Answers: []model.RR{{TTL: time.Hour, Data: "1"}}}
	q := model.Query{}
	zero := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionTTL, Value: chaos.TTLValueZero,
	}}}, q, nil)
	if zero.Answers[0].TTL != 0 {
		t.Fatalf("zero ttl=%s", zero.Answers[0].TTL)
	}
	set := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionTTL, Value: chaos.TTLValueSet, TTL: 5 * time.Second,
	}}}, q, nil)
	if set.Answers[0].TTL != 5*time.Second {
		t.Fatalf("set ttl=%s", set.Answers[0].TTL)
	}
	clamped := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionTTL, Value: chaos.TTLValueClamp, Min: 2 * time.Second, Max: 3 * time.Second,
	}}}, q, nil)
	if clamped.Answers[0].TTL != 3*time.Second {
		t.Fatalf("clamp ttl=%s", clamped.Answers[0].TTL)
	}
	huge := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionTTL, Value: chaos.TTLValueSet, TTL: maxWireTTL + time.Hour,
	}}}, q, nil)
	if huge.Answers[0].TTL != maxWireTTL {
		t.Fatalf("overflow ttl=%s", huge.Answers[0].TTL)
	}
	if base.Answers[0].TTL != time.Hour {
		t.Fatal("base ttl mutated")
	}
}

func TestAlternateAllowlistAndCNAMELoop(t *testing.T) {
	q := model.Query{Name: "a.example.", Type: model.TypeA, Class: model.ClassIN}
	base := model.Result{Answers: []model.RR{{
		Name: q.Name, Type: model.TypeA, Class: model.ClassIN, TTL: time.Second, Data: "192.0.2.1",
	}}}
	got := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionAlternate, Values: []string{"10.42.0.9"},
	}}}, q, nil)
	if len(got.Answers) != 1 || got.Answers[0].Data != "10.42.0.9" {
		t.Fatalf("alternate %+v", got.Answers)
	}

	loop := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionAlternate, Values: []string{"a.example."},
	}}}, q, nil)
	if len(loop.Answers) != 1 || loop.Answers[0].Data != "192.0.2.1" {
		t.Fatalf("self-CNAME must be skipped: %+v", loop.Answers)
	}
}

func TestPartialAnswerImmutability(t *testing.T) {
	base := model.Result{Answers: []model.RR{
		{Data: "192.0.2.1", Type: model.TypeA, TTL: time.Second},
		{Data: "192.0.2.2", Type: model.TypeA, TTL: time.Second},
		{Data: "192.0.2.3", Type: model.TypeA, TTL: time.Second},
	}}
	q := model.Query{Type: model.TypeA}
	got := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{
		{Type: model.ActionOmit, Values: []string{"192.0.2.2"}},
		{Type: model.ActionLimit, Limit: 1},
	}}, q, nil)
	if len(got.Answers) != 1 || got.Answers[0].Data != "192.0.2.1" {
		t.Fatalf("partial %+v", got.Answers)
	}
	if len(base.Answers) != 3 {
		t.Fatalf("base mutated len=%d", len(base.Answers))
	}
	shuf := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionShuffle, Seed: 42,
	}}}, q, nil)
	rot := ApplyResponse(base, chaos.ActionPlan{Actions: []chaos.PlannedAction{{
		Type: model.ActionRotate, Limit: 1,
	}}}, q, nil)
	if rot.Answers[0].Data != "192.0.2.2" {
		t.Fatalf("rotate %+v", rot.Answers)
	}
	if len(shuf.Answers) != 3 || len(base.Answers) != 3 {
		t.Fatal("shuffle mutated base")
	}
}
