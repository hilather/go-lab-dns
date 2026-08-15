package app

import (
	"context"
	"os"

	"github.com/hilather/go-lab-dns/internal/compiler"
	"github.com/hilather/go-lab-dns/internal/config"
	"github.com/hilather/go-lab-dns/internal/domainerr"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
)

// Reset rereads the bootstrap mount (or last bootstrap snapshot when no path
// is configured), compiles, and swaps only after success. It never writes the
// bootstrap file. A missing or invalid file leaves the active snapshot in
// place and does not clear the idempotency cache.
func (s *App) Reset(ctx context.Context, actor Actor, in ResetIn) (*ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.store.Load()
	gen := model.Generation(0)
	if prev != nil {
		gen = prev.Generation + 1
	}

	next, err := s.loadBootstrapCandidate(ctx, gen)
	if err != nil {
		return nil, err
	}

	displaced := s.store.Swap(next)
	// Bootstrap pointer tracks the last successfully compiled mount so a
	// later reset without a path can still restore it. Set after compile
	// succeeds so a bad file cannot replace Bootstrap.
	s.store.SetBootstrap(next)
	s.idemp.clear()

	diff, human, err := diffStates(canonicalOf(displaced), next.Canonical)
	if err != nil {
		return nil, err
	}
	_ = human
	cand := &candidate{
		prev:   displaced,
		next:   next,
		base:   canonicalOf(displaced),
		diff:   diff,
		impact: impactOf(canonicalOf(displaced), next.Canonical, diff),
	}
	res := &ApplyResult{
		Plan:       *s.planFrom(cand),
		Applied:    true,
		Generation: next.Generation,
	}
	res.AuditEventID = s.audit.append(AuditEvent{
		Time:       s.clock.Now(),
		ActorID:    actor.ID,
		Capability: "dns_state_reset",
		Reason:     in.Reason,
		Ticket:     in.Ticket,
		Revision:   next.Revision,
		Previous:   revisionOf(displaced),
	})
	return cloneApply(res), nil
}

func (s *App) loadBootstrapCandidate(ctx context.Context, gen model.Generation) (*snapshot.Snapshot, error) {
	if s.bootstrapPath != "" {
		if _, err := os.Stat(s.bootstrapPath); err != nil {
			if os.IsNotExist(err) {
				return nil, domainerr.ValidationFailed("bootstrap file unavailable",
					domainerr.FieldViolation{Path: "bootstrapPath", Code: "required", Message: "bootstrap file is missing; active snapshot unchanged"})
			}
			return nil, domainerr.Internal("stat bootstrap: " + err.Error())
		}
		st, err := config.LoadFile(s.bootstrapPath)
		if err != nil {
			return nil, asDomain(err)
		}
		snap, err := compiler.Compile(ctx, st, compiler.CompileOpts{
			Clock:      s.clock,
			Generation: gen,
		})
		if err != nil {
			return nil, asDomain(err)
		}
		return snap, nil
	}
	boot := s.store.Bootstrap()
	if boot == nil || boot.Canonical == nil {
		return nil, domainerr.ValidationFailed("no bootstrap snapshot",
			domainerr.FieldViolation{Path: "bootstrap", Code: "required", Message: "no bootstrap path or snapshot to reset to"})
	}
	copied, err := cloneState(boot.Canonical)
	if err != nil {
		return nil, err
	}
	snap, err := compiler.Compile(ctx, copied, compiler.CompileOpts{
		Clock:      s.clock,
		Generation: gen,
	})
	if err != nil {
		return nil, asDomain(err)
	}
	return snap, nil
}

func canonicalOf(s *snapshot.Snapshot) *model.State {
	if s == nil {
		return nil
	}
	return s.Canonical
}
