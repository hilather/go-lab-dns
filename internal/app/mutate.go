package app

import (
	"context"

	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

type candidate struct {
	prev      *snapshot.Snapshot
	next      *snapshot.Snapshot
	base      *model.State
	ops       []model.Operation
	diff      []DiffEntry
	impact    Impact
	humanDiff string
}

// Plan dry-runs the mutation pipeline. expectedRevision is required and must
// match the active runtime revision.
func (s *App) Plan(ctx context.Context, actor Actor, in ChangeIn) (*Plan, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planLocked(ctx, actor, in)
}

func (s *App) planLocked(ctx context.Context, actor Actor, in ChangeIn) (*Plan, error) {
	_ = actor
	fp, err := fingerprintChange(in)
	if err != nil {
		return nil, err
	}
	if hit, err := s.idemp.lookup(in.IdempotencyKey, fp); err != nil {
		return nil, err
	} else if hit != nil && hit.plan != nil {
		return clonePlan(hit.plan), nil
	}
	cand, err := s.buildCandidate(ctx, in, true)
	if err != nil {
		return nil, err
	}
	p := s.planFrom(cand)
	s.idemp.storePlan(in.IdempotencyKey, fp, p)
	return clonePlan(p), nil
}

// Apply compiles the candidate and atomically swaps only after success.
func (s *App) Apply(ctx context.Context, actor Actor, in ChangeIn) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyLocked(ctx, actor, in)
}

func (s *App) applyLocked(ctx context.Context, actor Actor, in ChangeIn) (*ApplyResult, error) {
	fp, err := fingerprintChange(in)
	if err != nil {
		return nil, err
	}
	if hit, err := s.idemp.lookup(in.IdempotencyKey, fp); err != nil {
		return nil, err
	} else if hit != nil && hit.apply != nil {
		return cloneApply(hit.apply), nil
	}
	cand, err := s.buildCandidate(ctx, in, true)
	if err != nil {
		return nil, err
	}
	prev := s.store.Swap(cand.next)
	res := &ApplyResult{
		Plan:       *s.planFrom(cand),
		Applied:    true,
		Generation: cand.next.Generation,
	}
	res.AuditEventID = s.audit.append(AuditEvent{
		Time:       s.clock.Now(),
		ActorID:    actor.ID,
		Capability: "dns_change_apply",
		Reason:     in.Reason,
		Ticket:     in.Ticket,
		Revision:   cand.next.Revision,
		Previous:   revisionOf(prev),
	})
	s.idemp.storeApply(in.IdempotencyKey, fp, res)
	return cloneApply(res), nil
}

// Validate inspects a candidate document and/or operations. It never swaps
// and does not require expectedRevision.
func (s *App) Validate(ctx context.Context, actor Actor, in ValidateIn) (*Plan, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = actor
	var base *model.State
	var prev *snapshot.Snapshot
	if in.State != nil {
		copied, err := cloneState(in.State)
		if err != nil {
			return nil, err
		}
		base = copied
		prev = s.store.Load()
	} else {
		snap, err := s.active()
		if err != nil {
			return nil, err
		}
		prev = snap
		copied, err := cloneState(snap.Canonical)
		if err != nil {
			return nil, err
		}
		base = copied
	}
	if err := applyOperations(base, in.Operations); err != nil {
		return nil, err
	}
	off := false
	if prev != nil {
		off = prev.EmergencyChaosOff
	}
	next, err := compileCandidate(ctx, base, prev, s.clock, off)
	if err != nil {
		return nil, asDomain(err)
	}
	before := baseForDiff(prev, in.State)
	diff, human, err := diffStates(before, next.Canonical)
	if err != nil {
		return nil, err
	}
	cand := &candidate{
		prev:      prev,
		next:      next,
		base:      before,
		ops:       append([]model.Operation(nil), in.Operations...),
		diff:      diff,
		impact:    impactOf(before, next.Canonical, diff),
		humanDiff: human,
	}
	return clonePlan(s.planFrom(cand)), nil
}

func baseForDiff(prev *snapshot.Snapshot, submitted *model.State) *model.State {
	if prev != nil && prev.Canonical != nil {
		if c, err := cloneState(prev.Canonical); err == nil {
			return c
		}
	}
	if submitted != nil {
		if c, err := cloneState(submitted); err == nil {
			return c
		}
	}
	return &model.State{}
}

func (s *App) buildCandidate(ctx context.Context, in ChangeIn, requireRev bool) (*candidate, error) {
	prev, err := s.active()
	if err != nil {
		return nil, err
	}
	if requireRev {
		if in.ExpectedRevision == "" {
			return nil, domainerr.ValidationFailed("expectedRevision is required",
				domainerr.FieldViolation{Path: "expectedRevision", Code: "required", Message: "expectedRevision is required for plan and apply"})
		}
		if in.ExpectedRevision != prev.Revision {
			return nil, domainerr.RevisionConflict("active revision does not match expectedRevision", string(prev.Revision)).
				WithRemediation("Re-read GET state and re-plan against the current revision.")
		}
	}
	copied, err := cloneState(prev.Canonical)
	if err != nil {
		return nil, err
	}
	if err := applyOperations(copied, in.Operations); err != nil {
		return nil, err
	}
	next, err := compileCandidate(ctx, copied, prev, s.clock, prev.EmergencyChaosOff)
	if err != nil {
		return nil, asDomain(err)
	}
	diff, human, err := diffStates(prev.Canonical, next.Canonical)
	if err != nil {
		return nil, err
	}
	return &candidate{
		prev:      prev,
		next:      next,
		base:      prev.Canonical,
		ops:       append([]model.Operation(nil), in.Operations...),
		diff:      diff,
		impact:    impactOf(prev.Canonical, next.Canonical, diff),
		humanDiff: human,
	}, nil
}

func (s *App) planFrom(c *candidate) *Plan {
	prevRev := model.Revision("")
	if c.prev != nil {
		prevRev = c.prev.Revision
	}
	boot := model.Revision("")
	if c.next != nil {
		boot = c.next.BootstrapRevision
	} else if c.prev != nil {
		boot = c.prev.BootstrapRevision
	}
	return &Plan{
		PreviousRevision:  prevRev,
		CandidateRevision: c.next.Revision,
		Drifted:           c.next.Revision != boot,
		Diff:              c.diff,
		Impact:            c.impact,
		Operations:        append([]model.Operation(nil), c.ops...),
		Auth:              AuthDecision{Allowed: true, Scopes: []string{"dns.write"}},
	}
}

func revisionOf(s *snapshot.Snapshot) model.Revision {
	if s == nil {
		return ""
	}
	return s.Revision
}
