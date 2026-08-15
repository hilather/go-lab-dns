package effects

import (
	"context"
	"errors"
	"sync"

	"github.com/hilather/go-lab-dns/internal/chaos"
	"github.com/hilather/go-lab-dns/internal/dnsserver"
	"github.com/hilather/go-lab-dns/internal/model"
	"github.com/hilather/go-lab-dns/internal/snapshot"
	"github.com/hilather/go-lab-dns/internal/testutil"
)

// Session holds one query's delay reservation and cancel watch.
// One reservation covers every delay phase of the request.
type Session struct {
	clk     testutil.Clock
	budgets *chaos.Budgets
	snap    *snapshot.Snapshot
	metrics *chaos.Metrics

	mu       sync.Mutex
	tok      *chaos.Token
	skipped  bool // budget exhausted
	stopped  bool // emergency: skip remaining delays
	cancelCh chan struct{}
	unreg    func()
}

// NewSession starts a cancellable delay session. Release must be called.
func NewSession(clk testutil.Clock, budgets *chaos.Budgets, snap *snapshot.Snapshot, metrics *chaos.Metrics) *Session {
	if clk == nil {
		clk = testutil.SystemClock{}
	}
	s := &Session{
		clk:      clk,
		budgets:  budgets,
		snap:     snap,
		metrics:  metrics,
		cancelCh: make(chan struct{}),
	}
	if budgets != nil {
		s.unreg = budgets.WatchCancel(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			select {
			case <-s.cancelCh:
			default:
				close(s.cancelCh)
			}
		})
	}
	return s
}

// Sleep waits for each delay action in phase. Budget exhaustion skips
// remaining delays without failing the query. Query-timeout
// (DeadlineExceeded) is not a cancel: the planned delay still runs so a
// 2s/10s chaos delay is not turned into a silent drop. Shutdown / peer
// cancel still aborts. Emergency CancelAll skips remaining delay and
// returns nil so the caller can send the base answer.
func (s *Session) Sleep(ctx context.Context, plan chaos.ActionPlan, phase string) error {
	if s == nil {
		return nil
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
	for _, a := range plan.Actions {
		if a.Delay <= 0 || a.Skip {
			continue
		}
		if a.Type != model.ActionDelay && !(a.Type == model.ActionUpstream && a.Phase == model.PhaseBeforeUpstream) {
			continue
		}
		if phase != "" && a.Phase != phase {
			continue
		}
		if err := s.sleepOne(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) sleepOne(ctx context.Context, a chaos.PlannedAction) error {
	s.mu.Lock()
	if s.skipped || s.stopped {
		s.mu.Unlock()
		return nil
	}
	if s.tok == nil && s.budgets != nil && s.snap != nil {
		maxG := s.snap.Safety.MaxConcurrentDelayed
		maxP := 0
		if s.snap.Chaos.ByID != nil {
			if cp, ok := s.snap.Chaos.ByID[a.PolicyID]; ok && cp != nil && cp.Policy.Budget != nil {
				maxP = cp.Policy.Budget.MaxConcurrency
			}
		}
		tok, err := s.budgets.ReserveDelay(a.PolicyID, maxG, maxP)
		if err != nil {
			s.skipped = true
			s.mu.Unlock()
			if s.metrics != nil {
				s.metrics.BudgetSkipped.Add(1)
			}
			return nil
		}
		s.tok = tok
	}
	s.mu.Unlock()

	timer := s.clk.NewTimer(a.Delay)
	defer timer.Stop()
	if s.metrics != nil {
		s.metrics.Delayed.Add(1)
	}
	var done <-chan struct{}
	if ctx != nil {
		done = ctx.Done()
	}
	select {
	case <-done:
		if ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			// Query budget elapsed; finish the planned delay, or abort
			// on emergency / server shutdown (not the query deadline).
			return s.waitDelayOrEmergency(ctx, timer)
		}
		if s.metrics != nil {
			s.metrics.DelayCanceled.Add(1)
		}
		return ctx.Err()
	case <-s.cancelCh:
		s.markStopped()
		return nil
	case <-timer.C():
		return nil
	}
}

func (s *Session) waitDelayOrEmergency(ctx context.Context, timer testutil.Timer) error {
	var shutdown <-chan struct{}
	if sc := dnsserver.ServerContext(ctx); sc != nil {
		shutdown = sc.Done()
	}
	select {
	case <-s.cancelCh:
		s.markStopped()
		return nil
	case <-timer.C():
		return nil
	case <-shutdown:
		if s.metrics != nil {
			s.metrics.DelayCanceled.Add(1)
		}
		if sc := dnsserver.ServerContext(ctx); sc != nil && sc.Err() != nil {
			return sc.Err()
		}
		return context.Canceled
	}
}

func (s *Session) markStopped() {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	if s.metrics != nil {
		s.metrics.DelayCanceled.Add(1)
	}
}

// Release drops the reservation and unregister the emergency watch.
func (s *Session) Release() {
	if s == nil {
		return
	}
	if s.unreg != nil {
		s.unreg()
		s.unreg = nil
	}
	s.mu.Lock()
	tok := s.tok
	s.tok = nil
	s.mu.Unlock()
	tok.Release()
}

// BudgetSkipped reports that a delay was omitted because the cap was hit.
func (s *Session) BudgetSkipped() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.skipped
}
