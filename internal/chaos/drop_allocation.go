package chaos

import (
	"math"

	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// Reserve a snapshot-wide union bound on every possible drop selector event.
// Activation, scope, weighted outcomes and composition can only reduce this
// bound. A policy uses one threshold across phases, but each drop-capable
// execution phase reserves a share: deterministic time buckets can change
// between phases, and not every Decide caller supplies a StickyRand.
func allocateDropProbabilities(index *snapshot.ChaosIndex, policies []model.ChaosPolicy, cap float64) {
	if cap <= 0 || cap >= 1 {
		return
	}
	total := 0.0
	phaseCount := 0
	for _, policy := range policies {
		phases := dropPhaseCount(policy)
		if policy.Selector.Probability > 0 {
			phaseCount += phases
		}
		total += policy.Selector.Probability * float64(phases)
	}
	if total <= cap {
		return
	}
	if phaseCount == 1 {
		for _, policy := range policies {
			if dropPhaseCount(policy) != 0 && policy.Selector.Probability > 0 {
				cp := index.ByID[policy.ID]
				cp.RequestedProbability = cp.Policy.Selector.Probability
				cp.Policy.Selector.Probability = math.Min(cp.Policy.Selector.Probability, cap)
			}
		}
		return
	}
	scale := math.Nextafter(cap/total, 0)
	remaining := cap
	for _, policy := range policies {
		phases := dropPhaseCount(policy)
		if phases == 0 {
			continue
		}
		probability := policy.Selector.Probability * scale
		// Configuration order makes rounding/remainder handling deterministic.
		probability = math.Min(probability, remaining/float64(phases))
		cp := index.ByID[policy.ID]
		cp.RequestedProbability = cp.Policy.Selector.Probability
		cp.Policy.Selector.Probability = probability
		remaining = math.Max(0, remaining-probability*float64(phases))
	}
}

func dropPhaseCount(policy model.ChaosPolicy) int {
	pre, response := false, false
	for _, outcome := range policy.Outcomes {
		if outcome.Weight <= 0 {
			continue
		}
		for _, action := range outcome.Actions {
			if action.Type != model.ActionDrop && (action.Type != model.ActionPressure || normalizeActionValue(action.Value) != PressureValueDrop) {
				continue
			}
			phase := actionPhaseOf(action)
			pre = pre || actionInPhase(phase, PhasePreResolution)
			response = response || actionInPhase(phase, PhaseResponse)
		}
	}
	count := 0
	if pre {
		count++
	}
	if response {
		count++
	}
	return count
}
