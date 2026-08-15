package effects

import (
	"math"
	"net/netip"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/model"
)

// maxWireTTL is the RFC 1035 TTL field (uint32 seconds).
const maxWireTTL = time.Duration(math.MaxUint32) * time.Second

// ApplyResponse copies res and applies rcode/TTL/answer/EDE actions in
// configured order. The stored snapshot RRset is never mutated.
func ApplyResponse(res model.Result, plan chaos.ActionPlan, q model.Query, metrics *chaos.Metrics) model.Result {
	out := copyResult(res)
	for _, a := range plan.Actions {
		switch a.Type {
		case model.ActionRCode:
			applyRCode(&out, a)
			if metrics != nil {
				metrics.RCodes.Add(1)
			}
		case model.ActionAlternate:
			applyAlternate(&out, a, q)
			if metrics != nil {
				metrics.Alternates.Add(1)
			}
		case model.ActionOmit:
			applyOmit(&out, a)
		case model.ActionLimit:
			applyLimit(&out, a)
		case model.ActionShuffle:
			applyShuffle(&out, a)
		case model.ActionRotate:
			applyRotate(&out, a)
		case model.ActionTTL:
			applyTTL(&out, a)
			if metrics != nil {
				metrics.TTL.Add(1)
			}
		}
	}
	if plan.EDE != nil && out.EDE == nil {
		e := *plan.EDE
		out.EDE = &e
	}
	return out
}

func applyRCode(res *model.Result, a chaos.PlannedAction) {
	v := strings.ToUpper(strings.TrimSpace(a.RCode))
	if v == "" {
		v = strings.ToUpper(strings.TrimSpace(a.Value))
	}
	switch v {
	case chaos.RCodeNODATA, "NOERROR":
		res.RCode = model.RCodeNoError
		if v == chaos.RCodeNODATA {
			res.Answers = nil
		}
	case string(model.RCodeServFail):
		res.RCode = model.RCodeServFail
		res.Answers = nil
	case string(model.RCodeRefused):
		res.RCode = model.RCodeRefused
		res.Answers = nil
	case string(model.RCodeNXDomain):
		res.RCode = model.RCodeNXDomain
		res.Answers = nil
	case string(model.RCodeFormErr):
		res.RCode = model.RCodeFormErr
		res.Answers = nil
	case string(model.RCodeNotImp):
		res.RCode = model.RCodeNotImp
		res.Answers = nil
	}
	if a.EDE != nil {
		e := *a.EDE
		res.EDE = &e
	}
}

func applyAlternate(res *model.Result, a chaos.PlannedAction, q model.Query) {
	if len(a.Values) == 0 {
		return
	}
	typ := q.Type
	if typ == "" && len(res.Answers) > 0 {
		typ = res.Answers[0].Type
	}
	name := q.Name
	if name == "" && len(res.Answers) > 0 {
		name = res.Answers[0].Name
	}
	ttl := time.Second
	if len(res.Answers) > 0 && res.Answers[0].TTL > 0 {
		ttl = res.Answers[0].TTL
	}
	class := model.ClassIN
	if q.Class != "" {
		class = q.Class
	}

	seen := map[string]struct{}{}
	for _, rr := range res.Answers {
		if rr.Type == model.TypeCNAME {
			seen[strings.ToLower(string(canonical(rr.Data)))] = struct{}{}
		}
	}
	seen[strings.ToLower(string(canonical(string(name))))] = struct{}{}

	out := make([]model.RR, 0, len(a.Values))
	for _, v := range a.Values {
		data := v
		useType := typ
		if _, err := netip.ParseAddr(v); err != nil {
			// Non-address values are CNAME targets when the query is not
			// already a CNAME-shaped replacement of A/AAAA RDATA.
			if typ != model.TypeCNAME && typ != model.TypeTXT && typ != model.TypeMX && typ != model.TypeSRV && typ != model.TypeNS && typ != model.TypePTR {
				useType = model.TypeCNAME
			}
			target := canonical(v)
			if _, loop := seen[strings.ToLower(string(target))]; loop {
				continue
			}
			seen[strings.ToLower(string(target))] = struct{}{}
			data = string(target)
		}
		out = append(out, model.RR{Name: name, Type: useType, Class: class, TTL: ttl, Data: data})
	}
	if len(out) == 0 {
		return
	}
	res.Answers = out
	if res.RCode == "" {
		res.RCode = model.RCodeNoError
	}
}

func applyOmit(res *model.Result, a chaos.PlannedAction) {
	if len(a.Values) == 0 || len(res.Answers) == 0 {
		return
	}
	drop := map[string]struct{}{}
	for _, v := range a.Values {
		drop[v] = struct{}{}
	}
	kept := res.Answers[:0]
	for _, rr := range res.Answers {
		if _, ok := drop[rr.Data]; ok {
			continue
		}
		kept = append(kept, rr)
	}
	res.Answers = kept
}

func applyLimit(res *model.Result, a chaos.PlannedAction) {
	if a.Limit < 0 || len(res.Answers) <= a.Limit {
		return
	}
	res.Answers = append([]model.RR(nil), res.Answers[:a.Limit]...)
}

func applyShuffle(res *model.Result, a chaos.PlannedAction) {
	n := len(res.Answers)
	if n < 2 {
		return
	}
	u := a.Seed
	if u == 0 {
		u = 1
	}
	for i := n - 1; i > 0; i-- {
		u = splitmix64(u)
		j := int(u % uint64(i+1))
		res.Answers[i], res.Answers[j] = res.Answers[j], res.Answers[i]
	}
}

func applyRotate(res *model.Result, a chaos.PlannedAction) {
	n := len(res.Answers)
	if n < 2 {
		return
	}
	k := a.Limit
	if k == 0 {
		k = 1
	}
	k %= n
	if k < 0 {
		k += n
	}
	res.Answers = append(append([]model.RR(nil), res.Answers[k:]...), res.Answers[:k]...)
}

func applyTTL(res *model.Result, a chaos.PlannedAction) {
	mode := strings.ToLower(strings.TrimSpace(a.Value))
	if mode == "" {
		if a.TTL == 0 && a.Min == 0 && a.Max == 0 {
			mode = chaos.TTLValueZero
		} else if a.Min != 0 || a.Max != 0 {
			if a.TTL != 0 {
				mode = chaos.TTLValueJitter
			} else {
				mode = chaos.TTLValueClamp
			}
		} else {
			mode = chaos.TTLValueSet
		}
	}
	apply := func(rrs []model.RR) []model.RR {
		if rrs == nil {
			return nil
		}
		out := append([]model.RR(nil), rrs...)
		for i := range out {
			out[i].TTL = transformTTL(out[i].TTL, mode, a)
		}
		return out
	}
	res.Answers = apply(res.Answers)
	res.Authority = apply(res.Authority)
	res.Additional = apply(res.Additional)
}

func transformTTL(cur time.Duration, mode string, a chaos.PlannedAction) time.Duration {
	var next time.Duration
	switch mode {
	case chaos.TTLValueZero:
		return 0
	case chaos.TTLValueSet:
		next = a.TTL
	case chaos.TTLValueClamp:
		next = cur
		if a.Min > 0 && next < a.Min {
			next = a.Min
		}
		if a.Max > 0 && next > a.Max {
			next = a.Max
		}
	case chaos.TTLValueJitter:
		spanMin, spanMax := a.Min, a.Max
		if spanMax < spanMin {
			spanMin, spanMax = spanMax, spanMin
		}
		j := chaos.UniformDelay(spanMin, spanMax, a.Seed)
		next = cur + j
	default:
		next = a.TTL
	}
	if next < 0 {
		next = 0
	}
	if next > maxWireTTL {
		next = maxWireTTL
	}
	return next
}

func copyResult(r model.Result) model.Result {
	if r.Answers != nil {
		r.Answers = append([]model.RR(nil), r.Answers...)
	}
	if r.Authority != nil {
		r.Authority = append([]model.RR(nil), r.Authority...)
	}
	if r.Additional != nil {
		r.Additional = append([]model.RR(nil), r.Additional...)
	}
	if r.WildcardSource != nil {
		v := *r.WildcardSource
		r.WildcardSource = &v
	}
	if r.ClosestEncloser != nil {
		v := *r.ClosestEncloser
		r.ClosestEncloser = &v
	}
	if r.EDE != nil {
		e := *r.EDE
		r.EDE = &e
	}
	if r.Explanation != nil {
		ex := *r.Explanation
		if ex.WildcardSource != nil {
			v := *ex.WildcardSource
			ex.WildcardSource = &v
		}
		if ex.ClosestEncloser != nil {
			v := *ex.ClosestEncloser
			ex.ClosestEncloser = &v
		}
		if ex.ChaosPolicyIDs != nil {
			ex.ChaosPolicyIDs = append([]model.PolicyID(nil), ex.ChaosPolicyIDs...)
		}
		if ex.ChaosActions != nil {
			ex.ChaosActions = append([]string(nil), ex.ChaosActions...)
		}
		r.Explanation = &ex
	}
	return r
}

func canonical(s string) model.Name {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "."
	}
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return model.Name(s)
}

func splitmix64(z uint64) uint64 {
	z += 0x9e3779b97f4a7c15
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
