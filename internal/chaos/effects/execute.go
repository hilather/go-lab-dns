package effects

import (
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/model"
)

// Annotate copies base-vs-final chaos fields onto the result explanation
// without recording raw QNAME labels beyond the existing query struct.
func Annotate(res *model.Result, base model.Result, plans ...chaos.ActionPlan) {
	if res == nil {
		return
	}
	if res.Explanation == nil {
		if base.Explanation != nil {
			ex := *base.Explanation
			res.Explanation = &ex
		} else {
			res.Explanation = &model.Explanation{}
		}
	}
	ex := res.Explanation
	if base.RCode != "" {
		ex.BaseRCode = base.RCode
	}
	var ids []model.PolicyID
	var acts []string
	for _, p := range plans {
		if p.Disabled {
			ex.ChaosDisabled = true
			if p.Reason != "" {
				ex.ChaosReason = p.Reason
			}
		}
		for _, d := range p.Decisions {
			if d.Triggered {
				ids = append(ids, d.PolicyID)
			}
		}
		for _, a := range p.Actions {
			acts = append(acts, a.Type)
		}
	}
	if len(ids) > 0 {
		ex.ChaosPolicyIDs = ids
	}
	if len(acts) > 0 {
		ex.ChaosActions = acts
	}
}
