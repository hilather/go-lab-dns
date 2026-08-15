package app

import (
	"context"
	"sort"
	"time"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
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
		EmergencyDisabled: snap.EmergencyChaosOff || s.store.EmergencyChaosOff() || snap.Canonical.Spec.Chaos.EmergencyDisabled,
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
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	q := model.Query{
		Name:      canonicalQueryName(in.Name),
		Type:      in.Type,
		Class:     in.Class,
		Client:    in.Client,
		Transport: in.Transport,
	}
	if q.Class == "" {
		q.Class = model.ClassIN
	}
	if q.Transport == "" {
		q.Transport = model.TransportUDP
	}
	if q.Type == "" {
		q.Type = model.TypeA
	}
	group := in.ClientGroup
	zone := in.ZoneID
	fwd := in.ForwardingID
	if group == "" && in.Client.IsValid() {
		group, _ = snap.Access.Classify(in.Client)
	}
	if zone == "" {
		zone, _ = snap.Zones.Select(q.Name)
	}
	if fwd == "" && group != "" {
		// Only select a forwarder when the caller did not classify; the
		// engine still consumes this ID and does not rediscover.
		if allow := groupAllowsForward(snap, group); allow {
			fwd, _ = snap.Forwarding.Select(q.Name)
		}
	}
	phase := chaos.Phase(in.Phase)
	ids := append([]model.PolicyID(nil), in.PolicyIDs...)
	if in.PolicyID != "" {
		ids = append(ids, in.PolicyID)
	}
	out, err := s.engine.Simulate(ctx, snap, chaos.SimulateIn{
		Query:         q,
		ClientGroupID: group,
		ZoneID:        zone,
		ForwardingID:  fwd,
		Base:          in.Base,
		Phase:         phase,
		Nonce:         in.Nonce,
		PolicyIDs:     ids,
	})
	if err != nil {
		return nil, asDomain(err)
	}
	decisions := make([]ChaosDecision, len(out.Decisions))
	for i, d := range out.Decisions {
		decisions[i] = ChaosDecision{
			PolicyID:   d.PolicyID,
			OutcomeID:  d.OutcomeID,
			Triggered:  d.Triggered,
			SkipReason: d.SkipReason,
			DigestHex:  d.Hash.DigestHex,
		}
	}
	return &SimulateOut{
		Algorithm: out.Algorithm,
		Disabled:  out.Disabled,
		Reason:    out.Reason,
		Triggered: out.Triggered,
		Decisions: decisions,
	}, nil
}

func groupAllowsForward(snap *snapshot.Snapshot, id model.ClientGroupID) bool {
	if snap == nil || snap.Canonical == nil || id == "" {
		return false
	}
	for _, g := range snap.Canonical.Spec.Access.ClientGroups {
		if g.ID == id {
			return g.AllowForward
		}
	}
	return false
}

func (s *App) ActivateChaos(ctx context.Context, actor Actor, in ActivationIn) (*ApplyResult, error) {
	return s.applyChaosActivation(ctx, actor, in.PolicyID, in.ExpectedRevision, in.IdempotencyKey, in.Reason, true, in.ExpiresAt, false)
}

func (s *App) DeactivateChaos(ctx context.Context, actor Actor, in ActivationIn) (*ApplyResult, error) {
	return s.applyChaosActivation(ctx, actor, in.PolicyID, in.ExpectedRevision, in.IdempotencyKey, in.Reason, false, in.ExpiresAt, false)
}

func (s *App) SetChaosExpiry(ctx context.Context, actor Actor, in ExpiryIn) (*ApplyResult, error) {
	return s.applyChaosActivation(ctx, actor, in.PolicyID, in.ExpectedRevision, in.IdempotencyKey, in.Reason, false, in.ExpiresAt, true)
}

func (s *App) applyChaosActivation(ctx context.Context, actor Actor, id model.PolicyID, rev model.Revision, key, reason string, enabled bool, exp *time.Time, expiryOnly bool) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, domainerr.ValidationFailed("policy id is required",
			domainerr.FieldViolation{Path: "policyId", Code: "required", Message: "policy id is required"})
	}
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	var current *model.ChaosPolicy
	for i := range snap.Canonical.Spec.Chaos.Policies {
		if snap.Canonical.Spec.Chaos.Policies[i].ID == id {
			p := snap.Canonical.Spec.Chaos.Policies[i]
			current = &p
			break
		}
	}
	if current == nil {
		return nil, domainerr.NotFound("chaos policy " + string(id) + " not found")
	}
	act := model.ChaosActivation{Enabled: enabled, ExpiresAt: exp}
	if expiryOnly {
		act.Enabled = current.Enabled
		act.ExpiresAt = exp
	} else if exp == nil {
		act.ExpiresAt = current.ExpiresAt
	}
	return s.Apply(ctx, actor, ChangeIn{
		ExpectedRevision: rev,
		IdempotencyKey:   key,
		Reason:           reason,
		Operations: []model.Operation{{
			Op:     model.OpUpdate,
			Target: model.Target{Kind: model.TargetChaosActivation, ID: string(id)},
			Value:  mustJSON(act),
		}},
	})
}

// EmergencyDisableChaos sets the store-level inhibit bit and CAS-stamps the
// current snapshot. It does not compile and does not take App.mu, so a long
// apply cannot block it. It never republishes a Canonical copied before a
// concurrent Swap. YAML emergencyDisabled still forces the bit on.
func (s *App) EmergencyDisableChaos(ctx context.Context, actor Actor, in EmergencyIn) (*ApplyResult, error) {
	return s.setEmergency(ctx, actor, in, true, "dns_chaos_emergency_disable")
}

// EmergencyEnableChaos clears the runtime inhibit bit. It cannot relax a
// YAML spec.chaos.emergencyDisabled=true value already on Canonical, and it
// cannot clear a startup --chaos-disable / LABDNS_CHAOS_DISABLE lock.
func (s *App) EmergencyEnableChaos(ctx context.Context, actor Actor, in EmergencyIn) (*ApplyResult, error) {
	return s.setEmergency(ctx, actor, in, false, "dns_chaos_emergency_enable")
}

func (s *App) setEmergency(ctx context.Context, actor Actor, in EmergencyIn, off bool, cap string) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	prev, err := s.active()
	if err != nil {
		return nil, err
	}
	s.store.SetEmergencyChaosOff(off)
	next := s.store.StampEmergency()
	if next == nil {
		return nil, domainerr.Internal("no active snapshot")
	}
	res := &ApplyResult{
		Plan: Plan{
			PreviousRevision:  revisionOf(prev),
			CandidateRevision: next.Revision,
			Drifted:           drifted(next),
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
		Previous:   revisionOf(prev),
	})
	return cloneApply(res), nil
}
