package app

import (
	"context"
	"sort"
	"time"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
)

func (s *App) ChaosStatus(ctx context.Context, actor Actor) (*ChaosRuntimeStatus, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	var nearest *time.Time
	active := 0
	for _, p := range snap.Canonical.Spec.Chaos.Policies {
		if p.Enabled {
			active++
			if p.ExpiresAt != nil && (nearest == nil || p.ExpiresAt.Before(*nearest)) {
				t := *p.ExpiresAt
				nearest = &t
			}
		}
	}
	return &ChaosRuntimeStatus{
		Enabled:           snap.Canonical.Spec.Chaos.Enabled,
		EmergencyDisabled: snap.EmergencyChaosOff || snap.Canonical.Spec.Chaos.EmergencyDisabled,
		ActivePolicies:    active,
		NearestExpiry:     nearest,
	}, nil
}

func (s *App) ListChaosPolicies(ctx context.Context, actor Actor) ([]model.ChaosPolicy, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	out := make([]model.ChaosPolicy, len(snap.Canonical.Spec.Chaos.Policies))
	for i, p := range snap.Canonical.Spec.Chaos.Policies {
		out[i] = copyChaosPolicy(p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *App) GetChaosPolicy(ctx context.Context, actor Actor, id model.PolicyID) (*model.ChaosPolicy, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	for _, p := range snap.Canonical.Spec.Chaos.Policies {
		if p.ID == id {
			copied := copyChaosPolicy(p)
			return &copied, nil
		}
	}
	return nil, domainerr.NotFound("chaos policy " + string(id) + " not found")
}

func (s *App) SimulateChaos(ctx context.Context, actor Actor, in SimulateIn) (*SimulateOut, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	_ = in
	return nil, domainerr.UnsupportedCapability("chaos simulate requires CHA-001")
}

func (s *App) ActivateChaos(ctx context.Context, actor Actor, in ActivationIn) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	_ = in
	return nil, domainerr.UnsupportedCapability("chaos activate requires CHA-001")
}

func (s *App) DeactivateChaos(ctx context.Context, actor Actor, in ActivationIn) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	_ = in
	return nil, domainerr.UnsupportedCapability("chaos deactivate requires CHA-001")
}

func (s *App) SetChaosExpiry(ctx context.Context, actor Actor, in ExpiryIn) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = actor
	_ = in
	return nil, domainerr.UnsupportedCapability("chaos set-expiry requires CHA-001")
}

// EmergencyDisableChaos copies the active Snapshot, sets EmergencyChaosOff,
// and swaps. It does not compile and does not wait on Plan/Apply. Canonical
// (and therefore Revision) is unchanged. YAML emergencyDisabled still forces
// the bit on.
func (s *App) EmergencyDisableChaos(ctx context.Context, actor Actor, in EmergencyIn) (*ApplyResult, error) {
	return s.setEmergency(ctx, actor, in, true, "dns_chaos_emergency_disable")
}

// EmergencyEnableChaos clears the runtime emergency bit. It cannot relax a
// YAML spec.chaos.emergencyDisabled=true value already on Canonical.
func (s *App) EmergencyEnableChaos(ctx context.Context, actor Actor, in EmergencyIn) (*ApplyResult, error) {
	return s.setEmergency(ctx, actor, in, false, "dns_chaos_emergency_enable")
}

func (s *App) setEmergency(ctx context.Context, actor Actor, in EmergencyIn, off bool, cap string) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	// Fast path: do not take App.mu so a long apply compile cannot block
	// emergency disable. Indexes and Canonical stay shared and immutable.
	prev, err := s.active()
	if err != nil {
		return nil, err
	}
	s.emergencyOff.Store(off)
	next := *prev
	yamlOff := prev.Canonical != nil && prev.Canonical.Spec.Chaos.EmergencyDisabled
	next.EmergencyChaosOff = off || yamlOff
	next.Generation = prev.Generation + 1
	displaced := s.store.Swap(&next)
	res := &ApplyResult{
		Plan: Plan{
			PreviousRevision:  revisionOf(displaced),
			CandidateRevision: next.Revision,
			Drifted:           drifted(&next),
			Auth:              AuthDecision{Allowed: true, Scopes: []string{"dns.chaos.emergency"}},
		},
		Applied:    true,
		Generation: next.Generation,
	}
	res.AuditEventID = s.audit.append(AuditEvent{
		Time:       s.clock.Now(),
		ActorID:    actor.ID,
		Capability: cap,
		Reason:     in.Reason,
		Revision:   next.Revision,
		Previous:   revisionOf(displaced),
	})
	return cloneApply(res), nil
}
