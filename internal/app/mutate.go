package app

import (
	"context"

	"github.com/hilather/go-lab-dns/internal/audit"
	"github.com/hilather/go-lab-dns/internal/auth"
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
	if snap, err := s.active(); err == nil {
		if err := auth.AuthorizeChange(actor, in.Operations, snap.Canonical); err != nil {
			s.recordDenied(ctx, actor, "dns_change_plan", err)
			return nil, err
		}
	}
	fp, err := fingerprintChange(in)
	if err != nil {
		return nil, err
	}
	if hit, err := s.idemp.lookup(in.IdempotencyKey, fp); err != nil {
		return nil, err
	} else if hit != nil && hit.plan != nil {
		if err := s.checkExpectedRevision(in); err != nil {
			s.forgetIdempOnConflict(in.IdempotencyKey, err)
			return nil, err
		}
		prev, err := s.active()
		if err != nil {
			return nil, err
		}
		if hit.plan.PreviousRevision == prev.Revision {
			return clonePlan(hit.plan), nil
		}
		// Foreign mutation moved the base; recompute against current.
	}
	cand, err := s.buildCandidate(ctx, in, true)
	if err != nil {
		s.forgetIdempOnConflict(in.IdempotencyKey, err)
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
	if snap, err := s.active(); err == nil {
		if err := auth.AuthorizeChange(actor, in.Operations, snap.Canonical); err != nil {
			s.recordDenied(ctx, actor, "dns_change_apply", err)
			return nil, err
		}
	}
	fp, err := fingerprintChange(in)
	if err != nil {
		return nil, err
	}
	if hit, err := s.idemp.lookup(in.IdempotencyKey, fp); err != nil {
		return nil, err
	} else if hit != nil && hit.apply != nil {
		if in.ExpectedRevision == "" {
			return nil, missingExpectedRevision()
		}
		return cloneApply(hit.apply), nil
	}
	cand, err := s.buildCandidate(ctx, in, true)
	if err != nil {
		s.forgetIdempOnConflict(in.IdempotencyKey, err)
		return nil, err
	}
	if s.afterCompile != nil {
		s.afterCompile()
	}
	// Swap stamps Store.EmergencyChaosOff onto next; apply cannot clear it.
	prev := s.store.Swap(cand.next)
	res := &ApplyResult{
		Plan:       *s.planFrom(cand),
		Applied:    true,
		Generation: cand.next.Generation,
	}
	res.AuditEventID = s.recordAudit(ctx, audit.Event{
		Time:       s.clock.Now(),
		ActorID:    actor.ID,
		ActorClass: actor.Class,
		Capability: "dns_change_apply",
		Reason:     in.Reason,
		Ticket:     in.Ticket,
		Revision:   cand.next.Revision,
		Previous:   revisionOf(prev),
		Result:     audit.ResultOK,
		Diff:       toAuditDiff(cand.diff),
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
	var current *model.State
	if in.State != nil {
		current = in.State
	} else if snap := s.store.Load(); snap != nil {
		current = snap.Canonical
	}
	if err := auth.AuthorizeChange(actor, in.Operations, current); err != nil {
		s.recordDenied(ctx, actor, "dns_state_validate", err)
		return nil, err
	}
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
	if s.store != nil && s.store.EmergencyChaosOff() {
		off = true
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
		if err := s.checkExpectedRevision(in); err != nil {
			return nil, err
		}
	}
	copied, err := cloneState(prev.Canonical)
	if err != nil {
		return nil, err
	}
	if err := applyOperations(copied, in.Operations); err != nil {
		return nil, err
	}
	off := prev.EmergencyChaosOff
	if s.store != nil && s.store.EmergencyChaosOff() {
		off = true
	}
	next, err := compileCandidate(ctx, copied, prev, s.clock, off)
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
	scopes := auth.RequiredPermissions(c.ops, c.base)
	p := &Plan{
		PreviousRevision:  prevRev,
		CandidateRevision: c.next.Revision,
		Drifted:           c.next.Revision != boot,
		Diff:              c.diff,
		Impact:            c.impact,
		Operations:        append([]model.Operation(nil), c.ops...),
		Auth:              AuthDecision{Allowed: true, Scopes: scopes},
	}
	p.Impact.RequiredPermissions = scopes
	return p
}

func (s *App) recordDenied(ctx context.Context, actor Actor, cap string, err error) {
	code := ""
	if de, ok := domainerr.As(err); ok {
		code = string(de.Code)
	}
	s.recordAudit(ctx, audit.Event{
		Time:       s.clock.Now(),
		ActorID:    actor.ID,
		ActorClass: actor.Class,
		Capability: cap,
		Result:     audit.ResultDenied,
		ErrorCode:  code,
	})
}

func toAuditDiff(in []DiffEntry) []audit.RedactedEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]audit.RedactedEntry, len(in))
	for i, d := range in {
		out[i] = audit.RedactedEntry{Path: d.Path, Op: d.Op, Before: d.Before, After: d.After}
	}
	return out
}

func missingExpectedRevision() error {
	return domainerr.ValidationFailed("expectedRevision is required",
		domainerr.FieldViolation{Path: "expectedRevision", Code: "required", Message: "expectedRevision is required for plan and apply"})
}

func (s *App) checkExpectedRevision(in ChangeIn) error {
	if in.ExpectedRevision == "" {
		return missingExpectedRevision()
	}
	prev, err := s.active()
	if err != nil {
		return err
	}
	if in.ExpectedRevision != prev.Revision {
		return domainerr.RevisionConflict("active revision does not match expectedRevision", string(prev.Revision)).
			WithRemediation("Re-read GET state and re-plan against the current revision.")
	}
	return nil
}

func (s *App) forgetIdempOnConflict(key string, err error) {
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeRevisionConflict {
		return
	}
	s.idemp.evict(key)
}

func revisionOf(s *snapshot.Snapshot) model.Revision {
	if s == nil {
		return ""
	}
	return s.Revision
}
