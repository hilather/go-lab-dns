package effects

import (
	"testing"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/model"
)

func TestPressureRejectsOverCap(t *testing.T) {
	p := chaos.NewPressure()
	now := time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)
	plan := chaos.ActionPlan{Pressure: chaos.PressurePlan{
		PolicyID: "p", MaxConc: 1, OnExceed: "SERVFAIL",
	}}
	a := CheckPressure(p, plan, now, nil)
	if a.Exceeded {
		t.Fatal("first acquire must pass")
	}
	b := CheckPressure(p, plan, now, nil)
	if !b.Exceeded || b.RCode != model.RCodeServFail {
		t.Fatalf("second %+v", b)
	}
	a.Release()
	c := CheckPressure(p, plan, now, nil)
	if c.Exceeded {
		t.Fatal("after release must pass")
	}
	c.Release()

	rate := chaos.ActionPlan{Pressure: chaos.PressurePlan{
		PolicyID: "q", MaxRate: 1, OnExceed: "drop",
	}}
	d := CheckPressure(p, rate, now, nil)
	if d.Exceeded {
		t.Fatal("first rate must pass")
	}
	e := CheckPressure(p, rate, now, nil)
	if !e.Drop {
		t.Fatalf("rate exceed %+v", e)
	}
	d.Release()
}
