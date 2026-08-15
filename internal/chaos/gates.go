package chaos

import (
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func scopeMatches(p model.ChaosPolicy, in DecisionIn, recs []model.RecordID, wildcard model.RecordID, pool model.PoolID) bool {
	s := p.Scope
	if len(s.QTypes) > 0 && !containsType(s.QTypes, in.Query.Type) {
		return false
	}
	if len(s.Transports) > 0 && !containsTransport(s.Transports, in.Query.Transport) {
		return false
	}
	if len(s.ClientGroups) > 0 && !containsGroup(s.ClientGroups, in.ClientGroupID) {
		return false
	}
	if len(s.Zones) > 0 && !containsZone(s.Zones, in.ZoneID) {
		return false
	}
	if len(s.ForwardingIDs) > 0 && !containsPolicy(s.ForwardingIDs, in.ForwardingID) {
		return false
	}
	if len(s.UpstreamPools) > 0 && !containsPool(s.UpstreamPools, pool) {
		return false
	}
	if len(s.Owners) > 0 && !containsName(s.Owners, in.Query.Name) {
		return false
	}
	if len(s.RecordIDs) > 0 && !intersectsRecord(s.RecordIDs, recs) {
		return false
	}
	if len(s.WildcardSourceIDs) > 0 && (wildcard == "" || !containsRecord(s.WildcardSourceIDs, wildcard)) {
		return false
	}
	return true
}

func gateSkip(p model.ChaosPolicy, now time.Time, safety snapshot.SafetyPolicy, compiledAt time.Time) string {
	if !p.Enabled {
		return "disabled"
	}
	if p.StartsAt != nil && now.Before(p.StartsAt.UTC()) {
		return "not_started"
	}
	if p.ExpiresAt != nil && !now.Before(p.ExpiresAt.UTC()) {
		return "expired"
	}
	// Duration-after-activation: StartsAt + DefaultMaxLifetime when no explicit expiry.
	if p.ExpiresAt == nil && p.StartsAt != nil && safety.DefaultMaxLifetime > 0 {
		if !now.Before(p.StartsAt.UTC().Add(safety.DefaultMaxLifetime)) {
			return "expired"
		}
	}
	_ = compiledAt
	if p.Selector.Period > 0 && p.Selector.Unhealthy > 0 {
		if flapHealthy(now, p.Selector) {
			return "flap_healthy"
		}
	}
	return ""
}

func flapHealthy(now time.Time, sel model.ChaosSelector) bool {
	// (now − phaseOffset) mod period; unhealthy window is the first Unhealthy.
	elapsed := now.UTC().Add(-sel.PhaseOffset).Sub(time.Unix(0, 0))
	if elapsed < 0 {
		// Keep a non-negative residue on the Unix timeline.
		mod := sel.Period
		elapsed = ((elapsed % mod) + mod) % mod
	} else {
		elapsed = elapsed % sel.Period
	}
	return elapsed >= sel.Unhealthy
}

func everyNthSkip(now time.Time, sel model.ChaosSelector, u0 uint64) string {
	n := sel.EveryNth
	if n <= 1 {
		return ""
	}
	var idx uint64
	if sel.TimeBucket >= time.Second {
		bsec := uint64(sel.TimeBucket / time.Second)
		sec := now.UTC().Unix()
		if sec < 0 {
			// Negative instants still need a stable bucket index.
			floored := floorDiv(sec, int64(bsec))
			if floored < 0 {
				idx = uint64(-floored)
			} else {
				idx = uint64(floored)
			}
		} else {
			idx = uint64(sec) / bsec
		}
	} else {
		idx = u0
	}
	if idx%uint64(n) != 0 {
		return "every_nth"
	}
	return ""
}

func actionPhaseOf(a model.ChaosAction) string {
	if a.Phase != "" {
		return a.Phase
	}
	switch a.Type {
	case model.ActionDelay:
		return model.PhaseBeforeResponse
	case model.ActionRCode, model.ActionTTL, model.ActionAlternate, model.ActionOmit, model.ActionLimit, model.ActionShuffle, model.ActionRotate:
		return model.PhaseBeforeResponse
	case model.ActionDrop, model.ActionTruncate, model.ActionTCPClose, model.ActionTCPReset:
		return model.PhaseBeforeResponse
	case model.ActionCache, model.ActionUpstream:
		return model.PhaseBeforeUpstream
	default:
		return a.Phase
	}
}

func actionInPhase(phase string, want Phase) bool {
	switch want {
	case PhasePreResolution:
		return phase == model.PhaseBeforeResolution || phase == model.PhaseBeforeUpstream
	case PhaseResponse:
		return phase == model.PhaseAfterUpstream || phase == model.PhaseBeforeResponse || phase == ""
	default:
		return true
	}
}

func isTransportAction(typ string) bool {
	switch typ {
	case model.ActionDrop, model.ActionTruncate, model.ActionTCPClose, model.ActionTCPReset:
		return true
	default:
		return false
	}
}

func containsType(xs []model.RRType, v model.RRType) bool {
	want := model.RRType(strings.ToUpper(string(v)))
	for _, x := range xs {
		if model.RRType(strings.ToUpper(string(x))) == want {
			return true
		}
	}
	return false
}

func containsTransport(xs []model.Transport, v model.Transport) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsGroup(xs []model.ClientGroupID, v model.ClientGroupID) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsZone(xs []model.ZoneID, v model.ZoneID) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsPolicy(xs []model.PolicyID, v model.PolicyID) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsPool(xs []model.PoolID, v model.PoolID) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsName(xs []model.Name, v model.Name) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsRecord(xs []model.RecordID, v model.RecordID) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func intersectsRecord(xs, ys []model.RecordID) bool {
	for _, x := range xs {
		if containsRecord(ys, x) {
			return true
		}
	}
	return false
}

func containsPolicyFilter(xs []model.PolicyID, v model.PolicyID) bool {
	if len(xs) == 0 {
		return true
	}
	return containsPolicy(xs, v)
}
