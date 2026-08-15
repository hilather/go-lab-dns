package effects

import (
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/model"
)

// PressureOutcome is the action taken when a policy QPS/concurrency cap
// is exceeded. Zero value means the query proceeds.
type PressureOutcome struct {
	Exceeded bool
	Drop     bool
	RCode    model.RCode
	Release  func()
}

// CheckPressure accounts the query against the policy-scoped table.
func CheckPressure(p *chaos.Pressure, plan chaos.ActionPlan, now time.Time, metrics *chaos.Metrics) PressureOutcome {
	out := PressureOutcome{Release: func() {}}
	if p == nil || plan.Pressure.PolicyID == "" {
		return out
	}
	if plan.Pressure.MaxRate <= 0 && plan.Pressure.MaxConc <= 0 {
		return out
	}
	rel, exceeded := p.Acquire(plan.Pressure.PolicyID, plan.Pressure.MaxConc, plan.Pressure.MaxRate, now)
	out.Release = rel
	if !exceeded {
		return out
	}
	out.Exceeded = true
	if metrics != nil {
		metrics.PressureReject.Add(1)
	}
	switch strings.ToUpper(strings.TrimSpace(plan.Pressure.OnExceed)) {
	case "DROP":
		out.Drop = true
	case string(model.RCodeRefused):
		out.RCode = model.RCodeRefused
	default:
		out.RCode = model.RCodeServFail
	}
	return out
}

// PressureHint is HintDrop when pressure selected silent drop.
func PressureHint(out PressureOutcome) dnsserver.TransportHint {
	if out.Drop {
		return dnsserver.HintDrop
	}
	return dnsserver.HintSend
}
