package effects

import (
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// Hint maps a planned transport action onto the listener hint.
// TCP drop is a bounded no-response then close. Truncate on TCP is
// left as HintTruncate; the listener falls back to Send.
func Hint(plan chaos.ActionPlan, tr model.Transport, metrics *chaos.Metrics) dnsserver.TransportHint {
	switch plan.TransportHint {
	case "drop":
		if metrics != nil {
			metrics.Drops.Add(1)
		}
		if tr == model.TransportTCP {
			if metrics != nil {
				metrics.Holds.Add(1)
			}
			return dnsserver.HintHoldThenClose
		}
		return dnsserver.HintDrop
	case "truncate":
		if metrics != nil {
			metrics.Truncations.Add(1)
		}
		return dnsserver.HintTruncate
	case "tcp-close":
		if metrics != nil {
			metrics.TCPCloses.Add(1)
		}
		if plan.Hold > 0 {
			if metrics != nil {
				metrics.Holds.Add(1)
			}
			return dnsserver.HintHoldThenClose
		}
		return dnsserver.HintTCPClose
	case "tcp-reset":
		if metrics != nil {
			metrics.TCPResets.Add(1)
		}
		return dnsserver.HintTCPReset
	case "hold-then-close":
		if metrics != nil {
			metrics.Holds.Add(1)
		}
		return dnsserver.HintHoldThenClose
	default:
		return dnsserver.HintSend
	}
}

// EarlyFailure is a synthetic result for pre-resolution RCODE / pressure.
func EarlyFailure(plan chaos.ActionPlan, q model.Query) model.Result {
	rc := plan.EarlyRCode
	if rc == "" {
		return model.Result{}
	}
	res := model.Result{RCode: rc, Source: model.SourceNegative}
	if plan.EDE != nil {
		e := *plan.EDE
		res.EDE = &e
	}
	if stringsEqualFold(string(rc), string(model.RCodeNoError)) {
		// NODATA: empty answer, no authority until a zone SOA is attached.
		res.Source = model.SourceNegative
	}
	_ = q
	return res
}

func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'a' && ca <= 'z' {
			ca -= 'a' - 'A'
		}
		if cb >= 'a' && cb <= 'z' {
			cb -= 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// SelectTransportPlan preserves policy precedence across pre-resolution and
// response phases, including configuration order within the same scope class.
func SelectTransportPlan(snap *snapshot.Snapshot, plans ...chaos.ActionPlan) chaos.ActionPlan {
	var only chaos.ActionPlan
	count := 0
	for _, plan := range plans {
		if plan.TransportHint != "" {
			only = plan
			count++
		}
	}
	if count <= 1 {
		return only
	}

	order := map[model.PolicyID]int{}
	if snap != nil && snap.Canonical != nil {
		for i, p := range snap.Canonical.Spec.Chaos.Policies {
			order[p.ID] = i
		}
	}
	var chosen chaos.ActionPlan
	bestPrecedence, bestOrder := int(^uint(0)>>1), int(^uint(0)>>1)
	for _, plan := range plans {
		if plan.TransportHint == "" {
			continue
		}
		for _, d := range plan.Decisions {
			if !d.Triggered {
				continue
			}
			for _, a := range d.Actions {
				switch a.Type {
				case model.ActionDrop, model.ActionTruncate, model.ActionTCPClose, model.ActionTCPReset:
				default:
					continue
				}
				idx, ok := order[d.PolicyID]
				if !ok {
					idx = int(^uint(0) >> 1)
				}
				if d.Precedence < bestPrecedence || (d.Precedence == bestPrecedence && idx < bestOrder) {
					chosen, bestPrecedence, bestOrder = plan, d.Precedence, idx
				}
			}
		}
	}
	return chosen
}
