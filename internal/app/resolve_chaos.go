package app

import (
	"context"
	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/chaos/effects"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

func (s *App) resolveAgainst(ctx context.Context, snap *snapshot.Snapshot, in ResolveIn) (model.Result, error) {
	base, err := s.resolveBaseAgainst(ctx, snap, in)
	if err != nil || !in.ApplyChaos {
		return base, err
	}
	query := base.Explanation.Query
	group := base.Explanation.ClientGroupID
	forwarding := model.PolicyID("")
	if groupAllowsForward(snap, group) {
		forwarding, _ = snap.Forwarding.Select(query.Name)
	}
	exclusive := chaos.NewExclusiveSet()
	simulate := func(phase chaos.Phase, result *model.Result) (chaos.ActionPlan, error) {
		if s.store.EmergencyChaosOff() {
			return chaos.ActionPlan{Disabled: true, Reason: "emergency_disabled"}, nil
		}
		modeled, err := s.engine.Simulate(ctx, snap, chaos.SimulateIn{Query: query, ClientGroupID: group, ZoneID: base.ZoneID, ForwardingID: forwarding, Base: result, Phase: phase, RespectActivation: true, Exclusive: exclusive})
		return modeled.Plan, err
	}
	pre, err := simulate(chaos.PhasePreResolution, nil)
	if err != nil {
		return model.Result{}, asDomain(err)
	}
	result := base
	if pre.SkipResolve && pre.EarlyRCode != "" {
		result = effects.EarlyFailure(pre, query)
		result.Explanation = base.Explanation
	}
	post, err := simulate(chaos.PhaseResponse, &result)
	if err != nil {
		return model.Result{}, asDomain(err)
	}
	result = effects.ApplyResponse(result, post, query, nil)
	effects.Annotate(&result, base, pre, post)
	result.Explanation.ForwardingID = forwarding
	result.Explanation.BaseAnswers = append([]model.RR(nil), base.Answers...)
	for _, plan := range []chaos.ActionPlan{pre, post} {
		for _, decision := range plan.Decisions {
			result.Explanation.ChaosDecisions = append(result.Explanation.ChaosDecisions, model.ChaosDecision{PolicyID: decision.PolicyID, OutcomeID: decision.OutcomeID, Triggered: decision.Triggered, SkipReason: decision.SkipReason, DigestHex: decision.Hash.DigestHex, Delay: decision.Delay()})
		}
	}
	return result, nil
}

func managementQuery(in ResolveIn) model.Query {
	query := model.Query{Name: canonicalQueryName(in.Name), Type: in.Type, Class: in.Class, Client: in.Client, Transport: in.Transport, RD: in.RD, CD: in.CD}
	if query.Type == "" {
		query.Type = model.TypeA
	}
	if query.Class == "" {
		query.Class = model.ClassIN
	}
	if query.Transport == "" {
		query.Transport = model.TransportUDP
	}
	return query
}
